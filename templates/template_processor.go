package templates

import (
	"text/template"
	"bytes"
	"github.com/gruntwork-io/boilerplate/errors"
	"os"
	"path/filepath"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/util"
	"io/ioutil"
	"path"
	"fmt"
	"github.com/gruntwork-io/boilerplate/variables"
	"reflect"
)

const MaxRenderAttempts = 15

// Process the boilerplate template specified in the given options and use the existing variables. This function will
// load any missing variables (either from command line options or by prompting the user), execute all the dependent
// boilerplate templates, and then execute this template. Note that we pass in rootOptions so that template dependencies
// can inspect properties of the root template.
func ProcessTemplate(options, rootOptions *config.BoilerplateOptions, thisDep variables.Dependency) error {
	rootBoilerplateConfig, err := config.LoadBoilerplateConfig(rootOptions)
	if err != nil {
		return err
	}

	boilerplateConfig, err := config.LoadBoilerplateConfig(options)
	if err != nil {
		return err
	}

	rawVars, err := config.GetVariables(options, boilerplateConfig, rootBoilerplateConfig, thisDep)
	if err != nil {
		return err
	}

	vars, err := renderVariables(rawVars, options)
	if err != nil {
		return err
	}

	err = os.MkdirAll(options.OutputFolder, 0777)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	err = processHooks(boilerplateConfig.Hooks.BeforeHooks, options, vars)
	if err != nil {
		return err
	}

	err = processDependencies(boilerplateConfig.Dependencies, options, vars)
	if err != nil {
		return err
	}

	err = processTemplateFolder(options, vars)
	if err != nil {
		return err
	}

	err = processHooks(boilerplateConfig.Hooks.AfterHooks, options, vars)
	if err != nil {
		return err
	}

	return nil
}

// Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so this function loops
// over each variable value, renders each one, and returns a new map of rendered variables.
func renderVariables(variables map[string]interface{}, options *config.BoilerplateOptions) (map[string]interface{}, error) {
	renderedVariables := map[string]interface{}{}

	for variableName, variableValue := range variables {
		rendered, err := renderVariable(variableValue, variables, options)
		if err != nil {
			return nil, err
		}
		renderedVariables[variableName] = rendered
	}

	return renderedVariables, nil
}

// Variable values are allowed to use Go templating syntax (e.g. to reference other variables), so here, we render
// those templates and return a new map of variables that are fully resolved.
func renderVariable(variable interface{}, variables map[string]interface{}, options *config.BoilerplateOptions) (interface{}, error) {
	valueType := reflect.ValueOf(variable)

	switch valueType.Kind() {
	case reflect.String:
		return renderTemplateRecursively(options.TemplateFolder, variable.(string), variables, options)
	case reflect.Slice:
		values := []interface{}{}
		for i := 0; i < valueType.Len(); i++ {
			rendered, err := renderVariable(valueType.Index(i).Interface(), variables, options)
			if err != nil {
				return  nil, err
			}
			values = append(values, rendered)
		}
		return values, nil
	case reflect.Map:
		values := map[interface{}]interface{}{}
		for _, key := range valueType.MapKeys() {
			renderedKey, err := renderVariable(key.Interface(), variables, options)
			if err != nil {
				return nil, err
			}
			renderedValue, err := renderVariable(valueType.MapIndex(key).Interface(), variables, options)
			if err != nil {
				return nil, err
			}
			values[renderedKey] = renderedValue
		}
		return values, nil
	default:
		return variable, nil
	}
}

// Process the given list of hooks, which are scripts that should be executed at the command-line
func processHooks(hooks []variables.Hook, options *config.BoilerplateOptions, vars map[string]interface{}) error {
	for _, hook := range hooks {
		err := processHook(hook, options, vars)
		if err != nil {
			return err
		}
	}

	return nil
}

// Process the given hook, which is a script that should be execute at the command-line
func processHook(hook variables.Hook, options *config.BoilerplateOptions, vars map[string]interface{}) error {
	cmd, err := renderTemplate(config.BoilerplateConfigPath(options.TemplateFolder), hook.Command, vars, options)
	if err != nil {
		return err
	}

	args := []string{}
	for _, arg := range hook.Args {
		renderedArg, err := renderTemplate(config.BoilerplateConfigPath(options.TemplateFolder), arg, vars, options)
		if err != nil {
			return err
		}
		args = append(args, renderedArg)
	}

	envVars := []string{}
	for key, value := range hook.Env {
		renderedKey, err := renderTemplate(config.BoilerplateConfigPath(options.TemplateFolder), key, vars, options)
		if err != nil {
			return err
		}

		renderedValue, err := renderTemplate(config.BoilerplateConfigPath(options.TemplateFolder), value, vars, options)
		if err != nil {
			return err
		}

		envVars = append(envVars, fmt.Sprintf("%s=%s", renderedKey, renderedValue))
	}

	return util.RunShellCommand(options.TemplateFolder, envVars, cmd, args...)
}

// Execute the boilerplate templates in the given list of dependencies
func processDependencies(dependencies []variables.Dependency, options *config.BoilerplateOptions, variables map[string]interface{}) error {
	for _, dependency := range dependencies {
		err := processDependency(dependency, options, variables)
		if err != nil {
			return err
		}
	}

	return nil
}

// Execute the boilerplate template in the given dependency
func processDependency(dependency variables.Dependency, options *config.BoilerplateOptions, variables map[string]interface{}) error {
	shouldProcess, err := shouldProcessDependency(dependency, options, variables)
	if err != nil {
		return err
	}

	if shouldProcess {
		dependencyOptions, err := cloneOptionsForDependency(dependency, options, variables)
		if err != nil {
			return err
		}

		util.Logger.Printf("Processing dependency %s, with template folder %s and output folder %s", dependency.Name, dependencyOptions.TemplateFolder, dependencyOptions.OutputFolder)
		return ProcessTemplate(dependencyOptions, options, dependency)
	} else {
		util.Logger.Printf("Skipping dependency %s", dependency.Name)
		return nil
	}
}

// Clone the given options for use when rendering the given dependency. The dependency will get the same options as
// the original passed in, except for the template folder, output folder, and command-line vars.
func cloneOptionsForDependency(dependency variables.Dependency, originalOptions *config.BoilerplateOptions, variables map[string]interface{}) (*config.BoilerplateOptions, error) {
	renderedTemplateFolder, err := renderTemplate(originalOptions.TemplateFolder, dependency.TemplateFolder, variables, originalOptions)
	if err != nil {
		return nil, err
	}
	renderedOutputFolder, err := renderTemplate(originalOptions.TemplateFolder, dependency.OutputFolder, variables, originalOptions)
	if err != nil {
		return nil, err
	}

	templateFolder := pathRelativeToTemplate(originalOptions.TemplateFolder, renderedTemplateFolder)
	outputFolder := pathRelativeToTemplate(originalOptions.OutputFolder, renderedOutputFolder)

	return &config.BoilerplateOptions{
		TemplateFolder: templateFolder,
		OutputFolder: outputFolder,
		NonInteractive: originalOptions.NonInteractive,
		Vars: cloneVariablesForDependency(dependency, variables),
		OnMissingKey: originalOptions.OnMissingKey,
		OnMissingConfig: originalOptions.OnMissingConfig,
	}, nil
}

// Clone the given variables for use when rendering the given dependency.  The dependency will get the same variables
// as the originals passed in, filtered to variable names that do not include a dependency or explicitly are for the
// given dependency. If dependency.DontInheritVariables is set to true, an empty map is returned.
func cloneVariablesForDependency(dependency variables.Dependency, originalVariables map[string]interface{}) map[string]interface{} {
	newVariables := map[string]interface{}{}

	if dependency.DontInheritVariables {
		return newVariables
	}

	for variableName, variableValue := range originalVariables {
		dependencyName, variableOriginalName := variables.SplitIntoDependencyNameAndVariableName(variableName)
		if dependencyName == dependency.Name {
			newVariables[variableOriginalName] = variableValue
		} else if _, alreadyExists := newVariables[variableName]; !alreadyExists {
			newVariables[variableName] = variableValue
		}
	}

	return newVariables
}

// Prompt the user to verify if the given dependency should be executed and return true if they confirm. If
// options.NonInteractive is set to true, this function always returns true.
func shouldProcessDependency(dependency variables.Dependency, options *config.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	shouldSkip, err := shouldSkipDependency(dependency, options, variables)
	if err != nil {
		return false, err
	}
	if shouldSkip {
		return false, nil
	}

	if options.NonInteractive {
		return true, nil
	}

	return util.PromptUserForYesNo(fmt.Sprintf("This boilerplate template has a dependency! Run boilerplate on dependency %s with template folder %s and output folder %s?", dependency.Name, dependency.TemplateFolder, dependency.OutputFolder))
}

// Return true if the skip parameter of the given dependency evaluates to a "true" value
func shouldSkipDependency(dependency variables.Dependency, options *config.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	if dependency.Skip == "" {
		return false, nil
	}

	rendered, err := renderTemplateRecursively(options.TemplateFolder, dependency.Skip, variables, options)
	if err != nil {
		return false, err
	}

	util.Logger.Printf("Skip attribute for dependency %s evaluated to '%s'", dependency.Name, rendered)
	return rendered == "true", nil
}

// Copy all the files and folders in templateFolder to outputFolder, passing text files through the Go template engine
// with the given set of variables as the data.
func processTemplateFolder(options *config.BoilerplateOptions, variables map[string]interface{}) error {
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", options.TemplateFolder, options.OutputFolder)

	return filepath.Walk(options.TemplateFolder, func(path string, info os.FileInfo, err error) error {
		if shouldSkipPath(path, options) {
			util.Logger.Printf("Skipping %s", path)
			return nil
		} else if util.IsDir(path) {
			return createOutputDir(path, options, variables)
		} else {
			return processFile(path, options, variables)
		}
	})
}

// Copy the given path, which is in the folder templateFolder, to the outputFolder, passing it through the Go template
// engine with the given set of variables as the data if it's a text file.
func processFile(path string, options *config.BoilerplateOptions, variables map[string]interface{}) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, options, variables)
	} else {
		return copyFile(path, options, variables)
	}
}

// Create the given directory, which is in templateFolder, in the given outputFolder
func createOutputDir(dir string, options *config.BoilerplateOptions, variables map[string]interface{}) error {
	destination, err := outPath(dir, options, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Creating folder %s", destination)
	return os.MkdirAll(destination, 0777)
}

// Compute the path where the given file, which is in templateFolder, should be copied in outputFolder. If the file
// path contains boilerplate syntax, use the given options and variables to render it to determine the final output
// path.
func outPath(file string, options *config.BoilerplateOptions, variables map[string]interface{}) (string, error) {
	templateFolderAbsPath, err := filepath.Abs(options.TemplateFolder)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	interpolatedFilePath, err := renderTemplate(file, file, variables, options)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	fileAbsPath, err := filepath.Abs(interpolatedFilePath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	relPath, err := filepath.Rel(templateFolderAbsPath, fileAbsPath)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return path.Join(options.OutputFolder, relPath), nil
}

// Copy the given file, which is in options.TemplateFolder, to options.OutputFolder
func copyFile(file string, options *config.BoilerplateOptions, variables map[string]interface{}) error {
	destination, err := outPath(file, options, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Copying %s to %s", file, destination)
	return util.CopyFile(file, destination)
}

// Run the template at templatePath, which is in templateFolder, through the Go template engine with the given
// variables as data and write the result to outputFolder
func processTemplate(templatePath string, options *config.BoilerplateOptions, variables map[string]interface{}) error {
	destination, err := outPath(templatePath, options, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Processing template %s and writing to %s", templatePath, destination)
	bytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	out, err := renderTemplate(templatePath, string(bytes), variables, options)
	if err != nil {
		return err
	}

	return util.WriteFileWithSamePermissions(templatePath, destination, []byte(out))
}

// Return true if this is a path that should not be copied
func shouldSkipPath(path string, options *config.BoilerplateOptions) bool {
	return path == options.TemplateFolder || path == config.BoilerplateConfigPath(options.TemplateFolder)
}

// Render the template at templatePath, with contents templateContents, using the Go template engine, passing in the
// given variables as data.
func renderTemplate(templatePath string, templateContents string, variables map[string]interface{}, options *config.BoilerplateOptions) (string, error) {
	option := fmt.Sprintf("missingkey=%s", string(options.OnMissingKey))
	tmpl := template.New(templatePath).Funcs(CreateTemplateHelpers(templatePath, options)).Option(option)

	parsedTemplate, err := tmpl.Parse(templateContents)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	var output bytes.Buffer
	if err := parsedTemplate.Execute(&output, variables); err != nil {
		return "", errors.WithStackTrace(err)
	}

	return output.String(), nil
}

// Render the template at templatePath, with contents templateContents, using the Go template engine, passing in the
// given variables as data. If the rendered result contains more Go templating syntax, render it again, and repeat this
// process recursively until there is no more rendering to be done.
//
// The main use case for this is to allow boilerplate variables to reference other boilerplate variables. This can
// obviously lead to an infinite loop. The proper way to prevent that would be to parse Go template syntax and build a
// dependency graph, but that is way too complicated. Therefore, we use hacky solution: render the template multiple
// times. If it is the same as the last time you rendered it, that means no new interpolations were processed, so
// we're done. If it changes, that means more interpolations are being processed, so keep going, up to a
// maximum number of render attempts.
func renderTemplateRecursively(templatePath string, templateContents string, variables map[string]interface{}, options *config.BoilerplateOptions) (string, error) {
	lastOutput := templateContents
	for i := 0; i < MaxRenderAttempts; i++ {
		output, err := renderTemplate(templatePath, lastOutput, variables, options)
		if err != nil {
			return "", err
		}

		if output == lastOutput {
			return output, nil
		}

		lastOutput = output
	}

	return "", errors.WithStackTrace(TemplateContainsInfiniteLoop{TemplatePath: templatePath, TemplateContents: templateContents, RenderAttempts: MaxRenderAttempts})
}

// Custom error types

type TemplateContainsInfiniteLoop struct {
	TemplatePath     string
	TemplateContents string
	RenderAttempts   int
}
func (err TemplateContainsInfiniteLoop) Error() string {
	return fmt.Sprintf("Template %s seems to contain infinite loop. After %d renderings, the contents continue to change. Template contents:\n%s", err.TemplatePath, err.RenderAttempts, err.TemplateContents)
}