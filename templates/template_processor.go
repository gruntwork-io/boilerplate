package templates

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/go-commons/collections"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/errors"
	getter_helper "github.com/gruntwork-io/boilerplate/getter-helper"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/util/prefixer"
	"github.com/gruntwork-io/boilerplate/variables"
)

// The name of the variable that contains the current value of the loop in each iteration of for_each
const eachVarName = "__each__"

// Process the boilerplate template specified in the given options and use the existing variables. This function will
// download remote templates to a temporary working directory, which is cleaned up at the end of the function. This
// function will load any missing variables (either from command line options or by prompting the user), execute all the
// dependent boilerplate templates, and then execute this template. Note that we pass in rootOptions so that template
// dependencies can inspect properties of the root template.
func ProcessTemplate(options, rootOpts *options.BoilerplateOptions, thisDep variables.Dependency) error {
	if options.Silent {
		options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		rootOpts.Logger = options.Logger
	} else if options.Logger == nil {
		options.Logger = slog.New(prefixer.New())
		rootOpts.Logger = options.Logger
	}

	// If TemplateFolder is already set, use that directly as it is a local template. Otherwise, download to a temporary
	// working directory.
	if options.TemplateFolder == "" {
		workingDir, templateFolder, err := getter_helper.DownloadTemplatesToTemporaryFolder(options.TemplateUrl, options.Logger)
		defer func() {
			options.Logger.Info(fmt.Sprintf("Cleaning up working directory."))
			os.RemoveAll(workingDir)
		}()
		if err != nil {
			return err
		}

		// Set the TemplateFolder of the options to the download dir
		options.TemplateFolder = templateFolder
	}

	rootBoilerplateConfig, err := config.LoadBoilerplateConfig(rootOpts)
	if err != nil {
		return err
	}
	if err := config.EnforceRequiredVersion(rootBoilerplateConfig); err != nil {
		return err
	}

	boilerplateConfig, err := config.LoadBoilerplateConfig(options)
	if err != nil {
		return err
	}
	if err := config.EnforceRequiredVersion(boilerplateConfig); err != nil {
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

	err = processDependencies(boilerplateConfig.Dependencies, options, boilerplateConfig.GetVariablesMap(), vars)
	if err != nil {
		return err
	}

	partials, err := processPartials(boilerplateConfig.Partials, options, vars)
	if err != nil {
		return err
	}

	err = processTemplateFolder(boilerplateConfig, options, vars, partials)
	if err != nil {
		return err
	}

	err = processHooks(boilerplateConfig.Hooks.AfterHooks, options, vars)
	if err != nil {
		return err
	}

	return nil
}

func processPartials(partials []string, opts *options.BoilerplateOptions, vars map[string]interface{}) ([]string, error) {
	var renderedPartials []string
	for _, partial := range partials {
		renderedPartial, err := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), partial, vars, opts)
		if err != nil {
			return []string{}, err
		}
		renderedPartials = append(renderedPartials, renderedPartial)
	}
	return renderedPartials, nil
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
		opts.Logger.Info(fmt.Sprintf("Skipping hook with command '%s'", hook.Command))
		return nil
	}

	cmd, err := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), hook.Command, vars, opts)
	if err != nil {
		return err
	}

	args := []string{}
	for _, arg := range hook.Args {
		renderedArg, err := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), arg, vars, opts)
		if err != nil {
			return err
		}
		args = append(args, renderedArg)
	}

	envVars := []string{}
	for key, value := range hook.Env {
		renderedKey, err := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), key, vars, opts)
		if err != nil {
			return err
		}

		renderedValue, err := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), value, vars, opts)
		if err != nil {
			return err
		}

		envVars = append(envVars, fmt.Sprintf("%s=%s", renderedKey, renderedValue))
	}

	workingDir := opts.TemplateFolder
	if hook.WorkingDir != "" {
		workingDir, err = render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), hook.WorkingDir, vars, opts)
		if err != nil {
			return err
		}
	}

	return util.RunShellCommand(workingDir, envVars, cmd, args...)
}

// Return true if the "skip" condition of this hook evaluates to true
func shouldSkipHook(hook variables.Hook, opts *options.BoilerplateOptions, vars map[string]interface{}) (bool, error) {
	if opts.DisableHooks {
		opts.Logger.Info(fmt.Sprintf("Hooks are disabled"))
		return true, nil
	}

	if hook.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateFromString(opts.TemplateFolder, hook.Skip, vars, opts)
	if err != nil {
		return false, err
	}

	opts.Logger.Info(fmt.Sprintf("Skip attribute for hook with command '%s' evaluated to '%s'", hook.Command, rendered))
	return rendered == "true", nil
}

// Execute the boilerplate templates in the given list of dependencies
func processDependencies(
	dependencies []variables.Dependency,
	opts *options.BoilerplateOptions,
	variablesInConfig map[string]variables.Variable,
	variables map[string]interface{},
) error {
	for _, dependency := range dependencies {
		err := processDependency(dependency, opts, variablesInConfig, variables)
		if err != nil {
			return err
		}
	}

	return nil
}

// Execute the boilerplate template in the given dependency
func processDependency(
	dependency variables.Dependency,
	opts *options.BoilerplateOptions,
	variablesInConfig map[string]variables.Variable,
	originalVars map[string]interface{},
) error {
	shouldProcess, err := shouldProcessDependency(dependency, opts, originalVars)
	if err != nil {
		return err
	}

	if shouldProcess {
		doProcess := func(updatedVars map[string]interface{}) error {
			dependencyOptions, err := cloneOptionsForDependency(dependency, opts, variablesInConfig, updatedVars)
			if err != nil {
				return err
			}

			opts.Logger.Info(fmt.Sprintf("Processing dependency %s, with template folder %s and output folder %s", dependency.Name, dependencyOptions.TemplateFolder, dependencyOptions.OutputFolder))
			return ProcessTemplate(dependencyOptions, opts, dependency)
		}

		forEach := dependency.ForEach
		if len(dependency.ForEachReference) > 0 {
			value, err := variables.UnmarshalListOfStrings(originalVars, dependency.ForEachReference)
			if err != nil {
				return err
			}

			forEach = value
		}

		if len(forEach) > 0 {
			for _, item := range forEach {
				updatedVars := collections.MergeMaps(originalVars, map[string]interface{}{eachVarName: item})
				if err := doProcess(updatedVars); err != nil {
					return err
				}
			}

			return nil
		} else {
			return doProcess(originalVars)
		}
	} else {
		opts.Logger.Info(fmt.Sprintf("Skipping dependency %s", dependency.Name))
		return nil
	}
}

// Clone the given options for use when rendering the given dependency. The dependency will get the same options as
// the original passed in, except for the template folder, output folder, and command-line vars.
func cloneOptionsForDependency(
	dependency variables.Dependency,
	originalOpts *options.BoilerplateOptions,
	variablesInConfig map[string]variables.Variable,
	variables map[string]interface{},
) (*options.BoilerplateOptions, error) {
	renderedTemplateUrl, err := render.RenderTemplateFromString(originalOpts.TemplateFolder, dependency.TemplateUrl, variables, originalOpts)
	if err != nil {
		return nil, err
	}
	renderedOutputFolder, err := render.RenderTemplateFromString(originalOpts.TemplateFolder, dependency.OutputFolder, variables, originalOpts)
	if err != nil {
		return nil, err
	}

	templateUrl, templateFolder, err := options.DetermineTemplateConfig(renderedTemplateUrl)
	if err != nil {
		return nil, err
	}
	// If local, make sure to return relative path in context of original template folder
	if templateFolder != "" {
		templateFolder = render.PathRelativeToTemplate(originalOpts.TemplateFolder, renderedTemplateUrl)
	}

	// Output folder should be local path relative to original output folder, or absolute path
	outputFolder := render.PathRelativeToTemplate(originalOpts.OutputFolder, renderedOutputFolder)

	renderedVarFiles := []string{}
	for _, varFilePath := range dependency.VarFiles {
		renderedVarFilePath, err := render.RenderTemplateFromString(originalOpts.TemplateFolder, varFilePath, variables, originalOpts)
		if err != nil {
			return nil, err
		}
		renderedVarFiles = append(renderedVarFiles, renderedVarFilePath)
	}

	vars, err := cloneVariablesForDependency(originalOpts, dependency, variablesInConfig, variables, renderedVarFiles)
	if err != nil {
		return nil, err
	}

	return &options.BoilerplateOptions{
		TemplateUrl:             templateUrl,
		TemplateFolder:          templateFolder,
		OutputFolder:            outputFolder,
		NonInteractive:          originalOpts.NonInteractive,
		Vars:                    vars,
		OnMissingKey:            originalOpts.OnMissingKey,
		OnMissingConfig:         originalOpts.OnMissingConfig,
		DisableHooks:            originalOpts.DisableHooks,
		DisableShell:            originalOpts.DisableShell,
		DisableDependencyPrompt: originalOpts.DisableDependencyPrompt,
	}, nil
}

// Clone the given variables for use when rendering the given dependency.  The dependency will get the same variables
// as the originals passed in, filtered to variable names that do not include a dependency or explicitly are for the
// given dependency.
// This function implements the following order of preference for rendering variables:
//   - Variables set on the CLI (originalVariables) directly for the dependency (DEPENDENCY.VARNAME), unless
//     DontInheritVariables is set.
//   - Variables defined from VarFiles set on the dependency.
//   - Variables defaults set on the dependency.
func cloneVariablesForDependency(
	opts *options.BoilerplateOptions,
	dependency variables.Dependency,
	variablesInConfig map[string]variables.Variable,
	originalVariables map[string]interface{},
	renderedVarFiles []string,
) (map[string]interface{}, error) {
	// Clone the opts so that we attempt to get the value for the variable, and we can error on any variable that is set
	// on a dependency and the value can't be computed.
	dependencyOpts := &options.BoilerplateOptions{
		NonInteractive: true,
		OnMissingKey:   options.ExitWithError,

		TemplateUrl:             opts.TemplateUrl,
		TemplateFolder:          opts.TemplateFolder,
		OutputFolder:            opts.OutputFolder,
		Vars:                    opts.Vars,
		OnMissingConfig:         opts.OnMissingConfig,
		DisableHooks:            opts.DisableHooks,
		DisableShell:            opts.DisableShell,
		DisableDependencyPrompt: opts.DisableDependencyPrompt,
		Logger:                  opts.Logger,
	}

	// Start with the original variables. Note that it doesn't matter that originalVariables contains both CLI and
	// non-CLI passed variables, as the CLI passed variables will be handled at the end to re-override back, ensuring
	// they have the highest precedence. Ideally we can handle non-CLI and CLI passed variables separately, but that
	// requires a larger refactoring at the top level so for now, we do this hacky approach allowing the dependency
	// defined variables to override both CLI and non-CLI passed variables, and then add back in the CLI passed
	// variables.
	// We also filter out any dependency namespaced variables, as those are only passed in from the CLI and will be
	// handled later.
	newVariables := map[string]interface{}{}
	if !dependency.DontInheritVariables {
		for key, value := range originalVariables {
			dependencyName, _ := variables.SplitIntoDependencyNameAndVariableName(key)
			if dependencyName == "" {
				newVariables[key] = value
			}
		}
	}

	varFileVars, err := variables.ParseVars(nil, renderedVarFiles)
	if err != nil {
		return nil, err
	}

	currentVariables := util.MergeMaps(originalVariables, varFileVars)
	for _, variable := range dependency.Variables {
		varValue, err := config.GetValueForVariable(
			variable,
			variablesInConfig,
			currentVariables,
			dependencyOpts,
			0,
		)
		if err != nil {
			return nil, err
		}
		newVariables[variable.Name()] = varValue
	}

	newVariables = util.MergeMaps(newVariables, varFileVars)

	if dependency.DontInheritVariables {
		return newVariables, nil
	}

	// Now handle the CLI passed variables. Note that we handle dependency namespaced values separately, as they have
	// the highest precedence.
	// First loop handling all variables that are not dependency namespaced, or that are dependency namespaced but are
	// not targeting this dependency.
	for key, value := range opts.Vars {
		dependencyName, _ := variables.SplitIntoDependencyNameAndVariableName(key)
		if dependencyName != dependency.Name {
			newVariables[key] = value
		}
	}
	// Second loop handling all variables that are dependency namespaced, overriding those that are not dependency
	// namespaced.
	for key, value := range opts.Vars {
		dependencyName, originalName := variables.SplitIntoDependencyNameAndVariableName(key)
		if dependencyName == dependency.Name {
			newVariables[originalName] = value
		}
	}

	return newVariables, nil
}

// Prompt the user to verify if the given dependency should be executed and return true if they confirm. If
// options.NonInteractive or options.DisableDependencyPrompt are set to true, this function always returns true.
func shouldProcessDependency(dependency variables.Dependency, opts *options.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	shouldSkip, err := shouldSkipDependency(dependency, opts, variables)
	if err != nil {
		return false, err
	}
	if shouldSkip {
		return false, nil
	}

	if opts.NonInteractive || opts.DisableDependencyPrompt {
		return true, nil
	}

	return util.PromptUserForYesNo(fmt.Sprintf("This boilerplate template has a dependency! Run boilerplate on dependency %s with template folder %s and output folder %s?", dependency.Name, dependency.TemplateUrl, dependency.OutputFolder))
}

// Return true if the skip parameter of the given dependency evaluates to a "true" value
func shouldSkipDependency(dependency variables.Dependency, opts *options.BoilerplateOptions, variables map[string]interface{}) (bool, error) {
	if dependency.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateFromString(opts.TemplateFolder, dependency.Skip, variables, opts)
	if err != nil {
		return false, err
	}

	opts.Logger.Info(fmt.Sprintf("Skip attribute for dependency %s evaluated to '%s'", dependency.Name, rendered))
	return rendered == "true", nil
}

// Copy all the files and folders in templateFolder to outputFolder, passing text files through the Go template engine
// with the given set of variables as the data.
func processTemplateFolder(
	config *config.BoilerplateConfig,
	opts *options.BoilerplateOptions,
	variables map[string]interface{},
	partials []string,
) error {
	opts.Logger.Info(fmt.Sprintf("Processing templates in %s and outputting generated files to %s", opts.TemplateFolder, opts.OutputFolder))

	// Process and render skip files and engines before walking so we only do the rendering operation once.
	processedSkipFiles, err := processSkipFiles(config.SkipFiles, opts, variables)
	if err != nil {
		return err
	}
	processedEngines, err := processEngines(config.Engines, opts, variables)
	if err != nil {
		return err
	}

	return filepath.Walk(opts.TemplateFolder, func(path string, info os.FileInfo, err error) error {
		path = filepath.ToSlash(path)
		if shouldSkipPath(path, opts, processedSkipFiles) {
			opts.Logger.Info(fmt.Sprintf("Skipping %s", path))
			return nil
		} else if util.IsDir(path) {
			return createOutputDir(path, opts, variables)
		} else {
			engine := determineTemplateEngine(processedEngines, path)
			return processFile(path, opts, variables, partials, engine)
		}
	})
}

// Copy the given path, which is in the folder templateFolder, to the outputFolder, passing it through the Go template
// engine with the given set of variables as the data if it's a text file.
func processFile(
	path string,
	opts *options.BoilerplateOptions,
	variables map[string]interface{},
	partials []string,
	engine variables.TemplateEngineType,
) error {
	isText, err := util.IsTextFile(path)
	if err != nil {
		return err
	}

	if isText {
		return processTemplate(path, opts, variables, partials, engine)
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

	opts.Logger.Info(fmt.Sprintf("Creating folder %s", destination))
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

	// | is an illegal filename char in Windows, so we also support urlencoded chars in the path. To support this, we
	// first urldecode the file before passing it through.
	urlDecodedFile, err := url.QueryUnescape(file)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	interpolatedFilePath, err := render.RenderTemplateFromString(file, urlDecodedFile, variables, opts)
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

	opts.Logger.Info(fmt.Sprintf("Copying %s to %s", file, destination))
	return util.CopyFile(file, destination)
}

// Run the template at templatePath, which is in templateFolder, through the Go template engine with the given
// variables as data and write the result to outputFolder
func processTemplate(
	templatePath string,
	opts *options.BoilerplateOptions,
	vars map[string]interface{},
	partials []string,
	engine variables.TemplateEngineType,
) error {
	destination, err := outPath(templatePath, opts, vars)
	if err != nil {
		return err
	}

	var out string
	switch engine {
	case variables.GoTemplate:
		out, err = render.RenderTemplateWithPartials(templatePath, partials, vars, opts)
		if err != nil {
			return err
		}
	case variables.Jsonnet:
		out, err = render.RenderJsonnetTemplate(templatePath, vars, opts)
		if err != nil {
			return err
		}
		// Strip the jsonnet extension from the destination, if it exists.
		destination = strings.TrimSuffix(destination, ".jsonnet")
	}

	return util.WriteFileWithSamePermissions(templatePath, destination, []byte(out))
}

// Return true if this is a path that should not be copied
func shouldSkipPath(path string, opts *options.BoilerplateOptions, processedSkipFiles []ProcessedSkipFile) bool {
	// Canonicalize paths for os portability.
	canonicalPath := filepath.ToSlash(path)
	canonicalTemplateFolder := filepath.ToSlash(opts.TemplateFolder)
	canonicalBoilerplateConfigPath := filepath.ToSlash(config.BoilerplateConfigPath(opts.TemplateFolder))

	// First check if the path is a part of the the skipFile list. To handle skipping:
	// - If the path matches with any entry in the skip files list and the if condition evaluates to true, skip the file.
	// - If the path does NOT match with any entry in the not_path list, and the if condition evaluates to true, skip
	//   the file.
	// NOTE: the composition for these directives are different. For `path` attribute, the composition is an `any`
	// operation. That is, if the path matches any one of the `path` attributes, then the file is skipped.
	// OTOH, for `not_path` attribute, the composition is an `all` operation. The file must not match ALL of the
	// `not_path` attributes to be skipped, but only if any one of the skip files has not_path attribute set.
	if pathInAnySkipPath(canonicalPath, processedSkipFiles) {
		return true
	}
	// not in any == not in all
	if anyNotPathEffective(processedSkipFiles) && pathInAnySkipNotPath(canonicalPath, processedSkipFiles) == false {
		return true
	}

	// Then check if the path is the template folder root or the boilerplate config.
	return canonicalPath == canonicalTemplateFolder || canonicalPath == canonicalBoilerplateConfigPath
}

// pathInAnySkipPath returns true if the given path matches any one of the path attributes in the skip file list.
func pathInAnySkipPath(canonicalPath string, skipFileList []ProcessedSkipFile) bool {
	for _, skipFile := range skipFileList {
		inSkipList := util.ListContains(canonicalPath, skipFile.EvaluatedPaths)
		if skipFile.RenderedSkipIf && inSkipList {
			return true
		}
	}
	return false
}

// anyNotPathEffective returns true if any skip file has a NotPath attribute defined and the rendered if condition is
// true.
func anyNotPathEffective(skipFileList []ProcessedSkipFile) bool {
	for _, skipFile := range skipFileList {
		if skipFile.RenderedSkipIf && len(skipFile.EvaluatedNotPaths) > 0 {
			return true
		}
	}
	return false
}

// pathInAnySkipNotPath returns true if the given path matches any one of the not_path attributes in the skip file list.
// Note that unlike pathInAnySkipPath, this also does a directory check, where the directory is considered in the
// not_path list. This is because the `not_path` list is an include list as opposed to a skip list, so we must copy over
// the directory to copy over the exact file.
func pathInAnySkipNotPath(canonicalPath string, skipFileList []ProcessedSkipFile) bool {
	for _, skipFile := range skipFileList {
		if skipFile.RenderedSkipIf == false || len(skipFile.EvaluatedNotPaths) == 0 {
			continue
		}

		inSkipNotPathList := util.ListContains(canonicalPath, skipFile.EvaluatedNotPaths)
		if inSkipNotPathList {
			return true
		}

		for _, path := range skipFile.EvaluatedNotPaths {
			if strings.HasPrefix(path, canonicalPath+"/") {
				return true
			}
		}
	}
	return false
}
