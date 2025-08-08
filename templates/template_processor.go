package templates

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/go-commons/collections"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/getterhelper"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

// The name of the variable that contains the current value of the loop in each iteration of for_each
const eachVarName = "__each__"

const defaultDirPerm = 0o777

// ProcessTemplate processes the boilerplate template specified in the given options and use the existing variables. This function will
// download remote templates to a temporary working directory, which is cleaned up at the end of the function. This
// function will load any missing variables (either from command line options or by prompting the user), execute all the
// dependent boilerplate templates, and then execute this template. Note that we pass in rootOptions so that template
// dependencies can inspect properties of the root template.
func ProcessTemplate(options, rootOpts *options.BoilerplateOptions, thisDep variables.Dependency) error {
	// If TemplateFolder is already set, use that directly as it is a local template. Otherwise, download to a temporary
	// working directory.
	if options.TemplateFolder == "" {
		workingDir, templateFolder, downloadErr := getterhelper.DownloadTemplatesToTemporaryFolder(options.TemplateURL)
		defer func() {
			util.Logger.Printf("Cleaning up working directory.")

			if rmErr := os.RemoveAll(workingDir); rmErr != nil {
				util.Logger.Printf("Failed to clean up working directory %s: %v", workingDir, rmErr)
			}
		}()

		if downloadErr != nil {
			return downloadErr
		}

		// Set the TemplateFolder of the options to the download dir
		options.TemplateFolder = templateFolder
	}

	rootBoilerplateConfig, rootCfgErr := config.LoadBoilerplateConfig(rootOpts)
	if rootCfgErr != nil {
		return rootCfgErr
	}

	if err := config.EnforceRequiredVersion(rootBoilerplateConfig); err != nil {
		return err
	}

	boilerplateConfig, cfgErr := config.LoadBoilerplateConfig(options)
	if cfgErr != nil {
		return cfgErr
	}

	if err := config.EnforceRequiredVersion(boilerplateConfig); err != nil {
		return err
	}

	vars, err := config.GetVariables(options, boilerplateConfig, rootBoilerplateConfig, thisDep)
	if err != nil {
		return err
	}

	err = os.MkdirAll(options.OutputFolder, defaultDirPerm)
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

func processPartials(partials []string, opts *options.BoilerplateOptions, vars map[string]any) ([]string, error) {
	renderedPartials := make([]string, 0, len(partials))

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
func processHooks(hooks []variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) error {
	if len(hooks) == 0 || opts.NoHooks {
		if opts.NoHooks {
			util.Logger.Printf("Hooks are disabled, skipping %d hook(s)", len(hooks))
		}

		return nil
	}

	executeAll := opts.NonInteractive // Auto-confirm all if non-interactive
	hookAnswers := make(map[string]bool)

	for _, hook := range hooks {
		skip, err := shouldSkipHook(hook, opts, vars)
		if err != nil || skip {
			if skip {
				util.Logger.Printf("Skipping hook with command '%s'", hook.Command)
			}

			if err != nil {
				return err
			}

			continue
		}

		hookDetails, err := renderHookDetails(hook, opts, vars)
		if err != nil {
			return err
		}

		hookKey := generateHookKey(hookDetails)

		// Check previous confirmation
		shouldExecute := handlePreviousHookConfirmation(hookKey, hookAnswers, executeAll)
		if !shouldExecute {
			continue
		}

		// Handle user confirmation if needed (skip if non-interactive)
		if !executeAll && !hookAnswers[hookKey] && !opts.NonInteractive {
			shouldExecute, shouldSetExecuteAll, err := handleHookUserConfirmation(hookDetails, hookKey, hookAnswers)
			if err != nil {
				return err
			}

			if !shouldExecute {
				continue
			}

			if shouldSetExecuteAll {
				executeAll = true
			}
		}

		// Execute the hook
		if err := processHook(hook, opts, vars); err != nil {
			return err
		}
	}

	return nil
}

// renderHookDetails renders the hook details and returns a pre-rendered string representation
func renderHookDetails(hook variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) (string, error) {
	base := config.BoilerplateConfigPath(opts.TemplateFolder)
	render := func(s string) (string, error) {
		return render.RenderTemplateFromString(base, s, vars, opts)
	}

	cmd, renderErr := render(hook.Command)
	if renderErr != nil {
		return "", renderErr
	}

	args := make([]string, len(hook.Args))

	for i, a := range hook.Args {
		if args[i], renderErr = render(a); renderErr != nil {
			return "", renderErr
		}
	}

	env := make([]string, 0, len(hook.Env))

	for k, v := range hook.Env {
		key, renderErr := render(k)
		if renderErr != nil {
			return "", renderErr
		}

		val, renderErr := render(v)
		if renderErr != nil {
			return "", renderErr
		}

		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	wd := opts.TemplateFolder

	if hook.WorkingDir != "" {
		var wdErr error
		if wd, wdErr = render(hook.WorkingDir); wdErr != nil {
			return "", wdErr
		}
	}

	// Create a user-friendly string representation for the hook details
	var details []string

	details = append(details, "Command: "+cmd)
	if len(args) > 0 {
		details = append(details, fmt.Sprintf("Arguments: %v", args))
	}

	if len(env) > 0 {
		details = append(details, fmt.Sprintf("Environment: %v", env))
	}

	details = append(details, "Working Directory: "+wd)

	hookDetails := strings.Join(details, "\n")

	return hookDetails, nil
}

// handlePreviousHookConfirmation checks if a hook was previously confirmed or declined
func handlePreviousHookConfirmation(hookKey string, hookAnswers map[string]bool, executeAll bool) bool {
	confirmed, seen := hookAnswers[hookKey]
	if !seen && !executeAll {
		return true
	}

	if seen && !confirmed {
		util.Logger.Printf("Skipping hook (previously declined)")
		return false
	}

	util.Logger.Printf("Executing hook (%s)", "previously confirmed or all confirmed")

	return true
}

// handleHookUserConfirmation prompts the user for confirmation and handles the response
func handleHookUserConfirmation(hookDetails string, hookKey string, hookAnswers map[string]bool) (bool, bool, error) {
	printHookDetails(hookDetails)

	resp, err := util.PromptUserForYesNoAll("Execute hook?")
	if err != nil {
		return false, false, err
	}

	switch resp {
	case util.UserResponseYes:
		hookAnswers[hookKey] = true

		util.Logger.Printf("Executing hook (user confirmed)")

		return true, false, nil // should execute, don't set executeAll
	case util.UserResponseAll:
		hookAnswers[hookKey] = true

		util.Logger.Printf("Executing hook (user confirmed all)")

		return true, true, nil // should execute, set executeAll
	case util.UserResponseNo:
		hookAnswers[hookKey] = false

		util.Logger.Printf("Skipping hook (user declined)")

		return false, false, nil // don't execute, don't set executeAll
	}

	return false, false, nil
}

// generateHookKey creates a unique key for a hook using a checksum of the hook details
func generateHookKey(hookDetails string) string {
	hash := sha256.Sum256([]byte(hookDetails))
	return fmt.Sprintf("hook_%x", hash)
}

// printHookDetails prints the details of a hook that will be executed
func printHookDetails(hookDetails string) {
	util.Logger.Printf("Hook details:")

	lines := strings.SplitSeq(hookDetails, "\n")
	for line := range lines {
		util.Logger.Printf("  %s", line)
	}
}

// Process the given hook, which is a script that should be execute at the command-line
func processHook(hook variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) error {
	cmd, hookRenderErr := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), hook.Command, vars, opts)
	if hookRenderErr != nil {
		return hookRenderErr
	}

	args := []string{}

	for _, arg := range hook.Args {
		renderedArg, hookRenderErr := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), arg, vars, opts)
		if hookRenderErr != nil {
			return hookRenderErr
		}

		args = append(args, renderedArg)
	}

	envVars := []string{}

	for key, value := range hook.Env {
		renderedKey, hookRenderErr := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), key, vars, opts)
		if hookRenderErr != nil {
			return hookRenderErr
		}

		renderedValue, hookRenderErr := render.RenderTemplateFromString(config.BoilerplateConfigPath(opts.TemplateFolder), value, vars, opts)
		if hookRenderErr != nil {
			return hookRenderErr
		}

		envVars = append(envVars, fmt.Sprintf("%s=%s", renderedKey, renderedValue))
	}

	workingDir := opts.TemplateFolder

	if hook.WorkingDir != "" {
		renderedWd, wdErr := render.RenderTemplateFromString(
			config.BoilerplateConfigPath(opts.TemplateFolder),
			hook.WorkingDir,
			vars,
			opts,
		)
		if wdErr != nil {
			return wdErr
		}

		workingDir = renderedWd
	}

	return util.RunShellCommand(workingDir, envVars, cmd, args...)
}

// Return true if the "skip" condition of this hook evaluates to true
func shouldSkipHook(hook variables.Hook, opts *options.BoilerplateOptions, vars map[string]interface{}) (bool, error) {
	if hook.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateFromString(opts.TemplateFolder, hook.Skip, vars, opts)
	if err != nil {
		return false, err
	}

	util.Logger.Printf("Skip attribute for hook with command '%s' evaluated to '%s'", hook.Command, rendered)

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

			util.Logger.Printf("Processing dependency %s, with template folder %s and output folder %s", dependency.Name, dependencyOptions.TemplateFolder, dependencyOptions.OutputFolder)

			return ProcessTemplate(dependencyOptions, opts, dependency)
		}

		forEach := dependency.ForEach

		if len(dependency.ForEachReference) > 0 {
			renderedReference, err := render.RenderTemplateFromString(opts.TemplateFolder, dependency.ForEachReference, originalVars, opts)
			if err != nil {
				return err
			}

			value, err := variables.UnmarshalListOfStrings(originalVars, renderedReference)
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
		util.Logger.Printf("Skipping dependency %s", dependency.Name)
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
	renderedTemplateURL, err := render.RenderTemplateFromString(originalOpts.TemplateFolder, dependency.TemplateURL, variables, originalOpts)
	if err != nil {
		return nil, err
	}

	renderedOutputFolder, err := render.RenderTemplateFromString(originalOpts.TemplateFolder, dependency.OutputFolder, variables, originalOpts)
	if err != nil {
		return nil, err
	}

	templateURL, templateFolder, err := options.DetermineTemplateConfig(renderedTemplateURL)
	if err != nil {
		return nil, err
	}
	// If local, make sure to return relative path in context of original template folder
	if templateFolder != "" {
		templateFolder = render.PathRelativeToTemplate(originalOpts.TemplateFolder, renderedTemplateURL)
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
		TemplateURL:             templateURL,
		TemplateFolder:          templateFolder,
		OutputFolder:            outputFolder,
		NonInteractive:          originalOpts.NonInteractive,
		Vars:                    vars,
		OnMissingKey:            originalOpts.OnMissingKey,
		OnMissingConfig:         originalOpts.OnMissingConfig,
		NoHooks:                 originalOpts.NoHooks,
		NoShell:                 originalOpts.NoShell,
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

		TemplateURL:             opts.TemplateURL,
		TemplateFolder:          opts.TemplateFolder,
		OutputFolder:            opts.OutputFolder,
		Vars:                    opts.Vars,
		OnMissingConfig:         opts.OnMissingConfig,
		NoHooks:                 opts.NoHooks,
		NoShell:                 opts.NoShell,
		DisableDependencyPrompt: opts.DisableDependencyPrompt,
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
		// If the value is a string, render it
		if strValue, ok := varValue.(string); ok {
			renderedValue, err := render.RenderTemplateFromString(opts.TemplateFolder, strValue, currentVariables, opts)
			if err != nil {
				return nil, err
			}

			varValue = renderedValue
		}

		newVariables[variable.Name()] = varValue
		// Update currentVariables to include the newly processed variable
		currentVariables = util.MergeMaps(currentVariables, map[string]interface{}{
			variable.Name(): varValue,
		})
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

	return util.PromptUserForYesNo(fmt.Sprintf("Process dependency '%s'?", dependency.Name))
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

	util.Logger.Printf("Skip attribute for dependency %s evaluated to '%s'", dependency.Name, rendered)

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
	util.Logger.Printf("Processing templates in %s and outputting generated files to %s", opts.TemplateFolder, opts.OutputFolder)

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

		switch {
		case shouldSkipPath(path, opts, processedSkipFiles):
			util.Logger.Printf("Skipping %s", path)
			return nil
		case util.IsDir(path):
			return createOutputDir(path, opts, variables)
		default:
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

	util.Logger.Printf("Creating folder %s", destination)

	return os.MkdirAll(destination, defaultDirPerm)
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

	util.Logger.Printf("Copying %s to %s", file, destination)

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
	if anyNotPathEffective(processedSkipFiles) && !pathInAnySkipNotPath(canonicalPath, processedSkipFiles) {
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
		if !skipFile.RenderedSkipIf || len(skipFile.EvaluatedNotPaths) == 0 {
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
