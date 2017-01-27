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
)

// Process the boilerplate template specified in the given options and use the existing variables. This function will
// load any missing variables (either from command line options or by prompting the user), execute all the dependent
// boilerplate templates, and then execute this template.
func ProcessTemplate(options *config.BoilerplateOptions) error {
	boilerplateConfig, err := config.LoadBoilerplateConfig(options)
	if err != nil {
		return err
	}

	vars, err := config.GetVariables(options, boilerplateConfig)
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

	err = processTemplateFolder(options, vars, boilerplateConfig.Dependencies)
	if err != nil {
		return err
	}

	err = processHooks(boilerplateConfig.Hooks.AfterHooks, options, vars)
	if err != nil {
		return err
	}

	return nil
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

	return util.RunShellCommand(options.TemplateFolder, cmd, args...)
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
	shouldProcess, err := shouldProcessDependency(dependency, options)
	if err != nil {
		return err
	}

	if shouldProcess {
		dependencyOptions := cloneOptionsForDependency(dependency, options, variables)

		util.Logger.Printf("Processing dependency %s, with template folder %s and output folder %s", dependency.Name, dependencyOptions.TemplateFolder, dependencyOptions.OutputFolder)
		return ProcessTemplate(dependencyOptions)
	} else {
		util.Logger.Printf("Skipping dependency %s", dependency.Name)
		return nil
	}
}

// Clone the given options for use when rendering the given dependency. The dependency will get the same options as
// the original passed in, except for the template folder, output folder, and command-line vars.
func cloneOptionsForDependency(dependency variables.Dependency, originalOptions *config.BoilerplateOptions, variables map[string]interface{}) *config.BoilerplateOptions {
	templateFolder := pathRelativeToTemplate(originalOptions.TemplateFolder, dependency.TemplateFolder)
	outputFolder := pathRelativeToTemplate(originalOptions.OutputFolder, dependency.OutputFolder)

	return &config.BoilerplateOptions{
		TemplateFolder: templateFolder,
		OutputFolder: outputFolder,
		NonInteractive: originalOptions.NonInteractive,
		Vars: cloneVariablesForDependency(dependency, variables),
		OnMissingKey: originalOptions.OnMissingKey,
		OnMissingConfig: originalOptions.OnMissingConfig,
	}
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
func shouldProcessDependency(dependency variables.Dependency, options *config.BoilerplateOptions) (bool, error) {
	if options.NonInteractive {
		return true, nil
	}

	return util.PromptUserForYesNo(fmt.Sprintf("This boilerplate template has a dependency! Run boilerplate on dependency %s with template folder %s and output folder %s?", dependency.Name, dependency.TemplateFolder, dependency.OutputFolder))
}

// Copy all the files and folders in templateFolder to outputFolder, passing text files through the Go template engine
// with the given set of variables as the data.
func processTemplateFolder(options *config.BoilerplateOptions, variables map[string]interface{}, rootDependencies []variables.Dependency) error {
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", options.TemplateFolder, options.OutputFolder)

	return filepath.Walk(options.TemplateFolder, func(path string, info os.FileInfo, err error) error {
		if shouldSkipPath(path, options) {
			util.Logger.Printf("Skipping %s", path)
			return nil
		} else if util.IsDir(path) {
			return createOutputDir(path, options, variables)
		} else {
			return processFile(path, options, variables, rootDependencies)
		}
	})
}

// Copy the given path, which is in the folder templateFolder, to the outputFolder, passing it through the Go template
// engine with the given set of variables as the data if it's a text file.
func processFile(path string, options *config.BoilerplateOptions, variables map[string]interface{}, rootDependencies []variables.Dependency) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, options, variables, rootDependencies)
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
func processTemplate(templatePath string, options *config.BoilerplateOptions, variables map[string]interface{}, rootDependencies []variables.Dependency) error {
	destination, err := outPath(templatePath, options, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Processing template %s and writing to %s", templatePath, destination)
	bytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	out, err := renderTemplate(templatePath, string(bytes), variables, options, rootDependencies)
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
func renderTemplate(templatePath string, templateContents string, variables map[string]interface{}, options *config.BoilerplateOptions, rootDependencies []variables.Dependency) (string, error) {
	option := fmt.Sprintf("missingkey=%s", string(options.OnMissingKey))
	tmpl := template.New(templatePath).Funcs(CreateTemplateHelpers(templatePath, options, rootDependencies)).Option(option)

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
