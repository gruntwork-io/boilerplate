package templates

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

// Process the boilerplate template specified in the given options and use the existing variables. This function will
// load any missing variables (either from command line options or by prompting the user), execute all the dependent
// boilerplate templates, and then execute this template. Note that we pass in rootOptions so that template dependencies
// can inspect properties of the root template.
func ProcessTemplate(options, rootOpts *options.BoilerplateOptions, thisDep variables.Dependency) error {
	rootBoilerplateConfig, err := config.LoadBoilerplateConfig(rootOpts)
	if err != nil {
		return err
	}

	boilerplateConfig, err := config.LoadBoilerplateConfig(options)
	if err != nil {
		return err
	}

	vars, err := config.GetVariables(options, boilerplateConfig, rootBoilerplateConfig, thisDep)
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

// Process the given list of hooks, which are scripts that should be executed at the command-line
func processHooks(hooks []variables.Hook, opts *options.BoilerplateOptions, vars map[string]interface{}) error {
	for _, hook := range hooks {
		err := processHook(hook, opts, vars)
		if err != nil {
			return err
		}
	}

	return nil
}

// Process the given hook, which is a script that should be execute at the command-line
func processHook(hook variables.Hook, opts *options.BoilerplateOptions, vars map[string]interface{}) error {
	skip, err := shouldSkipHook(hook, opts, vars)
	if err != nil {
		return err
	}
	if skip {
		util.Logger.Printf("Skipping hook with command '%s'", hook.Command)
		return nil
	}

	cmd, err := render.RenderTemplate(config.BoilerplateConfigPath(opts.TemplateFolder), hook.Command, vars, opts)
	if err != nil {
		return err
	}

	args := []string{}
	for _, arg := range hook.Args {
		renderedArg, err := render.RenderTemplate(config.BoilerplateConfigPath(opts.TemplateFolder), arg, vars, opts)
		if err != nil {
			return err
		}
		args = append(args, renderedArg)
	}

	envVars := []string{}
	for key, value := range hook.Env {
		renderedKey, err := render.RenderTemplate(config.BoilerplateConfigPath(opts.TemplateFolder), key, vars, opts)
		if err != nil {
			return err
		}

		renderedValue, err := render.RenderTemplate(config.BoilerplateConfigPath(opts.TemplateFolder), value, vars, opts)
		if err != nil {
			return err
		}

		envVars = append(envVars, fmt.Sprintf("%s=%s", renderedKey, renderedValue))
	}

	return util.RunShellCommand(opts.TemplateFolder, envVars, cmd, args...)
}

// Return true if the "skip" condition of this hook evaluates to true
func shouldSkipHook(hook variables.Hook, opts *options.BoilerplateOptions, vars map[string]interface{}) (bool, error) {
	if opts.DisableHooks {
		util.Logger.Printf("Hooks are disabled")
		return true, nil
	}

	if hook.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateRecursively(opts.TemplateFolder, hook.Skip, vars, opts)
	if err != nil {
		return false, err
	}

	util.Logger.Printf("Skip attribute for hook with command '%s' evaluated to '%s'", hook.Command, rendered)
	return rendered == "true", nil
}

// Execute the boilerplate templates in the given list of dependencies
func processDependencies(dependencies []variables.Dependency, opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	for _, dependency := range dependencies {
		err := processDependency(dependency, opts, variables)
		if err != nil {
			return err
		}
	}

	return nil
}

// Execute the boilerplate template in the given dependency
func processDependency(dependency variables.Dependency, opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	shouldProcess, err := shouldProcessDependency(dependency, opts, variables)
	if err != nil {
		return err
	}

	if shouldProcess {
		dependencyOptions, err := cloneOptionsForDependency(dependency, opts, variables)
		if err != nil {
			return err
		}

		util.Logger.Printf("Processing dependency %s, with template folder %s and output folder %s", dependency.Name, dependencyOptions.TemplateFolder, dependencyOptions.OutputFolder)
		return ProcessTemplate(dependencyOptions, opts, dependency)
	} else {
		util.Logger.Printf("Skipping dependency %s", dependency.Name)
		return nil
	}
}

// Clone the given options for use when rendering the given dependency. The dependency will get the same options as
// the original passed in, except for the template folder, output folder, and command-line vars.
func cloneOptionsForDependency(dependency variables.Dependency, originalOpts *options.BoilerplateOptions, variables map[string]interface{}) (*options.BoilerplateOptions, error) {
	renderedTemplateFolder, err := render.RenderTemplate(originalOpts.TemplateFolder, dependency.TemplateFolder, variables, originalOpts)
	if err != nil {
		return nil, err
	}
	renderedOutputFolder, err := render.RenderTemplate(originalOpts.TemplateFolder, dependency.OutputFolder, variables, originalOpts)
	if err != nil {
		return nil, err
	}

	templateFolder := render.PathRelativeToTemplate(originalOpts.TemplateFolder, renderedTemplateFolder)
	outputFolder := render.PathRelativeToTemplate(originalOpts.OutputFolder, renderedOutputFolder)

	return &options.BoilerplateOptions{
		TemplateFolder:  templateFolder,
		OutputFolder:    outputFolder,
		NonInteractive:  originalOpts.NonInteractive,
		Vars:            cloneVariablesForDependency(dependency, variables),
		OnMissingKey:    originalOpts.OnMissingKey,
		OnMissingConfig: originalOpts.OnMissingConfig,
		DisableHooks:    originalOpts.DisableHooks,
		DisableShell:    originalOpts.DisableShell,
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
func shouldProcessDependency(dependency variables.Dependency, opts *options.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	shouldSkip, err := shouldSkipDependency(dependency, opts, variables)
	if err != nil {
		return false, err
	}
	if shouldSkip {
		return false, nil
	}

	if opts.NonInteractive {
		return true, nil
	}

	return util.PromptUserForYesNo(fmt.Sprintf("This boilerplate template has a dependency! Run boilerplate on dependency %s with template folder %s and output folder %s?", dependency.Name, dependency.TemplateFolder, dependency.OutputFolder))
}

// Return true if the skip parameter of the given dependency evaluates to a "true" value
func shouldSkipDependency(dependency variables.Dependency, opts *options.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	if dependency.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateRecursively(opts.TemplateFolder, dependency.Skip, variables, opts)
	if err != nil {
		return false, err
	}

	util.Logger.Printf("Skip attribute for dependency %s evaluated to '%s'", dependency.Name, rendered)
	return rendered == "true", nil
}

// Copy all the files and folders in templateFolder to outputFolder, passing text files through the Go template engine
// with the given set of variables as the data.
func processTemplateFolder(opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", opts.TemplateFolder, opts.OutputFolder)

	return filepath.Walk(opts.TemplateFolder, func(path string, info os.FileInfo, err error) error {
		if shouldSkipPath(path, opts) {
			util.Logger.Printf("Skipping %s", path)
			return nil
		} else if util.IsDir(path) {
			return createOutputDir(path, opts, variables)
		} else {
			return processFile(path, opts, variables)
		}
	})
}

// Copy the given path, which is in the folder templateFolder, to the outputFolder, passing it through the Go template
// engine with the given set of variables as the data if it's a text file.
func processFile(path string, opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, opts, variables)
	} else {
		return copyFile(path, opts, variables)
	}
}

// Create the given directory, which is in templateFolder, in the given outputFolder
func createOutputDir(dir string, opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	destination, err := outPath(dir, opts, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Creating folder %s", destination)
	return os.MkdirAll(destination, 0777)
}

// Compute the path where the given file, which is in templateFolder, should be copied in outputFolder. If the file
// path contains boilerplate syntax, use the given options and variables to render it to determine the final output
// path.
func outPath(file string, opts *options.BoilerplateOptions, variables map[string]interface{}) (string, error) {
	templateFolderAbsPath, err := filepath.Abs(opts.TemplateFolder)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	interpolatedFilePath, err := render.RenderTemplate(file, file, variables, opts)
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

	return path.Join(opts.OutputFolder, relPath), nil
}

// Copy the given file, which is in options.TemplateFolder, to options.OutputFolder
func copyFile(file string, opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	destination, err := outPath(file, opts, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Copying %s to %s", file, destination)
	return util.CopyFile(file, destination)
}

// Run the template at templatePath, which is in templateFolder, through the Go template engine with the given
// variables as data and write the result to outputFolder
func processTemplate(templatePath string, opts *options.BoilerplateOptions, variables map[string]interface{}) error {
	destination, err := outPath(templatePath, opts, variables)
	if err != nil {
		return err
	}

	util.Logger.Printf("Processing template %s and writing to %s", templatePath, destination)
	bytes, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	out, err := render.RenderTemplate(templatePath, string(bytes), variables, opts)
	if err != nil {
		return err
	}

	return util.WriteFileWithSamePermissions(templatePath, destination, []byte(out))
}

// Return true if this is a path that should not be copied
func shouldSkipPath(path string, opts *options.BoilerplateOptions) bool {
	return path == opts.TemplateFolder || path == config.BoilerplateConfigPath(opts.TemplateFolder)
}
