package templates

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/getterhelper"
	"github.com/gruntwork-io/boilerplate/internal/fileutil"
	"github.com/gruntwork-io/boilerplate/internal/logging"
	"github.com/gruntwork-io/boilerplate/internal/manifest"
	"github.com/gruntwork-io/boilerplate/internal/shell"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/prompt"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/util"
	"github.com/gruntwork-io/boilerplate/variables"
)

// ProcessResult holds the outputs of a template processing run.
type ProcessResult struct {
	Variables      map[string]any
	Dependencies   []manifest.ManifestDependency
	SourceChecksum string
	GeneratedFiles []string
}

// The name of the variable that contains the current value of the loop in each iteration of for_each
const eachVarName = "__each__"

const defaultDirPerm = 0o777

// ProcessTemplate processes the boilerplate template specified in the given options and use the existing variables. This function will
// download remote templates to a temporary working directory, which is cleaned up at the end of the function. This
// function will load any missing variables (either from command line options or by prompting the user), execute all the
// dependent boilerplate templates, and then execute this template. Note that we pass in rootOptions so that template
// dependencies can inspect properties of the root template.
func ProcessTemplate(options, rootOpts *options.BoilerplateOptions, thisDep *variables.Dependency) error {
	_, err := ProcessTemplateWithContext(context.Background(), options, rootOpts, thisDep)
	return err
}

// ProcessTemplateWithContext is like ProcessTemplate but accepts a context for cancellation and timeouts.
// Returns a ProcessResult containing the list of generated file paths and source checksum.
func ProcessTemplateWithContext(ctx context.Context, options, rootOpts *options.BoilerplateOptions, thisDep *variables.Dependency) (*ProcessResult, error) {
	cleanup, cloneDir, err := resolveTemplate(options)
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		return nil, err
	}

	// Compute source checksum while the working directory (and clone dir) still exist.
	var sourceChecksum string

	if options.Manifest {
		sourceChecksum = computeSourceChecksum(options.TemplateFolder, cloneDir)
	}

	rootBoilerplateConfig, rootCfgErr := config.LoadBoilerplateConfig(rootOpts)
	if rootCfgErr != nil {
		return nil, rootCfgErr
	}

	if err := config.EnforceRequiredVersion(rootBoilerplateConfig); err != nil {
		return nil, err
	}

	boilerplateConfig, cfgErr := config.LoadBoilerplateConfig(options)
	if cfgErr != nil {
		return nil, cfgErr
	}

	if err := config.EnforceRequiredVersion(boilerplateConfig); err != nil {
		return nil, err
	}

	vars, err := config.GetVariablesWithContext(ctx, options, boilerplateConfig, rootBoilerplateConfig, thisDep)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(options.OutputFolder, defaultDirPerm)
	if err != nil {
		return nil, err
	}

	err = processHooks(ctx, boilerplateConfig.Hooks.BeforeHooks, options, vars)
	if err != nil {
		return nil, err
	}

	deps, err := processDependencies(ctx, boilerplateConfig.Dependencies, options, boilerplateConfig.GetVariablesMap(), vars)
	if err != nil {
		return nil, err
	}

	partials, err := processPartials(ctx, boilerplateConfig.Partials, options, vars)
	if err != nil {
		return nil, err
	}

	generatedFilePaths, err := processTemplateFolder(ctx, boilerplateConfig, options, vars, partials)
	if err != nil {
		return nil, err
	}

	err = processHooks(ctx, boilerplateConfig.Hooks.AfterHooks, options, vars)
	if err != nil {
		return nil, err
	}

	// Filter out builtin variables so the manifest only records user-defined ones.
	userVars := make(map[string]any, len(vars))
	for k, v := range vars {
		switch k {
		case "BoilerplateConfigVars", "BoilerplateConfigDeps", "This":
			continue
		default:
			userVars[k] = v
		}
	}

	return &ProcessResult{
		GeneratedFiles: generatedFilePaths,
		SourceChecksum: sourceChecksum,
		Variables:      userVars,
		Dependencies:   deps,
	}, nil
}

// resolveTemplate ensures opts.TemplateFolder is set, downloading remote
// templates if necessary. It returns a cleanup function (nil for local templates)
// and the clone directory (empty for local templates). The cleanup function must
// be deferred by the caller before checking the error.
func resolveTemplate(opts *options.BoilerplateOptions) (cleanup func(), cloneDir string, err error) {
	if opts.TemplateFolder != "" {
		return nil, "", nil
	}

	workingDir, templateFolder, downloadErr := getterhelper.DownloadTemplatesToTemporaryFolder(opts.TemplateURL)

	cleanup = func() {
		logging.Logger.Printf("Cleaning up working directory.")

		if rmErr := os.RemoveAll(workingDir); rmErr != nil {
			logging.Logger.Printf("Failed to clean up working directory %s: %v", workingDir, rmErr)
		}
	}

	if downloadErr != nil {
		return cleanup, "", downloadErr
	}

	opts.TemplateFolder = templateFolder

	return cleanup, filepath.Join(workingDir, getterhelper.CloneSubdir), nil
}

// computeSourceChecksum computes the source checksum, logging a warning on
// error and returning an empty string.
func computeSourceChecksum(templateDir, cloneDir string) string {
	cs, err := manifest.ComputeSourceChecksum(templateDir, cloneDir)
	if err != nil {
		logging.Logger.Printf("Warning: failed to compute source checksum: %v", err)

		return ""
	}

	return cs
}

func processPartials(ctx context.Context, partials []string, opts *options.BoilerplateOptions, vars map[string]any) ([]string, error) {
	renderedPartials := make([]string, 0, len(partials))

	for _, partial := range partials {
		renderedPartial, err := render.RenderTemplateFromStringWithContext(ctx, config.BoilerplateConfigPath(opts.TemplateFolder), partial, vars, opts)
		if err != nil {
			return []string{}, err
		}

		renderedPartials = append(renderedPartials, renderedPartial)
	}

	return renderedPartials, nil
}

// processHooks processes the given list of hooks, which are scripts that should be executed at the command-line
func processHooks(ctx context.Context, hooks []variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) error {
	if len(hooks) == 0 || opts.NoHooks {
		if opts.NoHooks {
			logging.Logger.Printf("Hooks are disabled, skipping %d hook(s)", len(hooks))
		}

		return nil
	}

	executeAll := opts.NonInteractive // Auto-confirm all if non-interactive
	hookAnswers := make(map[string]bool)

	for i := range hooks {
		hook := &hooks[i]

		skip, err := shouldSkipHook(ctx, hook, opts, vars)
		if err != nil || skip {
			if skip {
				logging.Logger.Printf("Skipping hook with command '%s'", hook.Command)
			}

			if err != nil {
				return err
			}

			continue
		}

		hookDetails, err := renderHookDetails(ctx, hook, opts, vars)
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
		if err := processHook(ctx, hook, opts, vars); err != nil {
			return err
		}
	}

	return nil
}

// renderHookDetails renders the hook details and returns a pre-rendered string representation
func renderHookDetails(ctx context.Context, hook *variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) (string, error) {
	base := config.BoilerplateConfigPath(opts.TemplateFolder)
	render := func(s string) (string, error) {
		return render.RenderTemplateFromStringWithContext(ctx, base, s, vars, opts)
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
		logging.Logger.Printf("Skipping hook (previously declined)")
		return false
	}

	logging.Logger.Printf("Executing hook (%s)", "previously confirmed or all confirmed")

	return true
}

// handleHookUserConfirmation prompts the user for confirmation and handles the response
func handleHookUserConfirmation(hookDetails string, hookKey string, hookAnswers map[string]bool) (bool, bool, error) {
	printHookDetails(hookDetails)

	resp, err := prompt.PromptUserForYesNoAll("Execute hook?")
	if err != nil {
		return false, false, err
	}

	switch resp {
	case prompt.UserResponseYes:
		hookAnswers[hookKey] = true

		logging.Logger.Printf("Executing hook (user confirmed)")

		return true, false, nil // should execute, don't set executeAll
	case prompt.UserResponseAll:
		hookAnswers[hookKey] = true

		logging.Logger.Printf("Executing hook (user confirmed all)")

		return true, true, nil // should execute, set executeAll
	case prompt.UserResponseNo:
		hookAnswers[hookKey] = false

		logging.Logger.Printf("Skipping hook (user declined)")

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
	logging.Logger.Printf("Hook details:")

	lines := strings.SplitSeq(hookDetails, "\n")
	for line := range lines {
		logging.Logger.Printf("  %s", line)
	}
}

// processHook processes the given hook, which is a script that should be execute at the command-line
func processHook(ctx context.Context, hook *variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) error {
	cmd, hookRenderErr := render.RenderTemplateFromStringWithContext(ctx, config.BoilerplateConfigPath(opts.TemplateFolder), hook.Command, vars, opts)
	if hookRenderErr != nil {
		return hookRenderErr
	}

	args := []string{}

	for _, arg := range hook.Args {
		renderedArg, hookRenderErr := render.RenderTemplateFromStringWithContext(ctx, config.BoilerplateConfigPath(opts.TemplateFolder), arg, vars, opts)
		if hookRenderErr != nil {
			return hookRenderErr
		}

		args = append(args, renderedArg)
	}

	envVars := []string{}

	for key, value := range hook.Env {
		renderedKey, hookRenderErr := render.RenderTemplateFromStringWithContext(ctx, config.BoilerplateConfigPath(opts.TemplateFolder), key, vars, opts)
		if hookRenderErr != nil {
			return hookRenderErr
		}

		renderedValue, hookRenderErr := render.RenderTemplateFromStringWithContext(ctx, config.BoilerplateConfigPath(opts.TemplateFolder), value, vars, opts)
		if hookRenderErr != nil {
			return hookRenderErr
		}

		envVars = append(envVars, fmt.Sprintf("%s=%s", renderedKey, renderedValue))
	}

	workingDir := opts.TemplateFolder

	if hook.WorkingDir != "" {
		renderedWd, wdErr := render.RenderTemplateFromStringWithContext(
			ctx,
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

	return shell.RunShellCommandWithContext(ctx, workingDir, envVars, cmd, args...)
}

// Return true if the "skip" condition of this hook evaluates to true
func shouldSkipHook(ctx context.Context, hook *variables.Hook, opts *options.BoilerplateOptions, vars map[string]any) (bool, error) {
	if hook.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateFromStringWithContext(ctx, opts.TemplateFolder, hook.Skip, vars, opts)
	if err != nil {
		return false, err
	}

	logging.Logger.Printf("Skip attribute for hook with command '%s' evaluated to '%s'", hook.Command, rendered)

	return rendered == "true", nil
}

// processDependencies executes the boilerplate templates in the given list of dependencies
func processDependencies(
	ctx context.Context,
	dependencies []variables.Dependency,
	opts *options.BoilerplateOptions,
	variablesInConfig map[string]variables.Variable,
	variables map[string]any,
) ([]manifest.ManifestDependency, error) {
	var allDeps []manifest.ManifestDependency

	for i := range dependencies {
		deps, err := processDependency(ctx, &dependencies[i], opts, variablesInConfig, variables)
		if err != nil {
			return nil, err
		}

		allDeps = append(allDeps, deps...)
	}

	return allDeps, nil
}

// processDependency processes a single dependency and returns manifest entries for it.
// A single dependency with for_each can produce multiple entries.
func processDependency(
	ctx context.Context,
	dependency *variables.Dependency,
	opts *options.BoilerplateOptions,
	variablesInConfig map[string]variables.Variable,
	originalVars map[string]any,
) ([]manifest.ManifestDependency, error) {
	shouldProcess, err := shouldProcessDependency(ctx, dependency, opts, originalVars)
	if err != nil {
		return nil, err
	}

	if !shouldProcess {
		logging.Logger.Printf("Skipping dependency %s", dependency.Name)

		// Record skipped dependency with the rendered skip expression.
		renderedSkip, skipErr := render.RenderTemplateFromStringWithContext(ctx, opts.TemplateFolder, dependency.Skip, originalVars, opts)
		if skipErr != nil {
			renderedSkip = dependency.Skip
		}

		return []manifest.ManifestDependency{{
			Name:                 dependency.Name,
			TemplateURL:          dependency.TemplateURL,
			OutputFolder:         dependency.OutputFolder,
			Skip:                 renderedSkip,
			ForEach:              dependency.ForEach,
			ForEachReference:     dependency.ForEachReference,
			DontInheritVariables: dependency.DontInheritVariables,
		}}, nil
	}

	var allDeps []manifest.ManifestDependency
	var mu sync.Mutex

	doProcess := func(ctx context.Context, updatedVars map[string]any, forEach []string) error {
		dependencyOptions, cloneErr := cloneOptionsForDependency(ctx, dependency, opts, variablesInConfig, updatedVars)
		if cloneErr != nil {
			return cloneErr
		}

		logging.Logger.Printf("Processing dependency %s, with template folder %s and output folder %s", dependency.Name, dependencyOptions.TemplateFolder, dependencyOptions.OutputFolder)

		depResult, processErr := ProcessTemplateWithContext(ctx, dependencyOptions, opts, dependency)
		if processErr != nil {
			return processErr
		}

		// Use the dependency's result variables, which already have builtins filtered out.
		resolvedVars := maps.Clone(depResult.Variables)

		// Compute checksums for files generated by this dependency.
		var depFiles []manifest.GeneratedFile

		if opts.Manifest {
			for _, relPath := range depResult.GeneratedFiles {
				absPath := filepath.Join(dependencyOptions.OutputFolder, relPath)

				checksum, csErr := manifest.SHA256File(absPath)
				if csErr != nil {
					return csErr
				}

				depFiles = append(depFiles, manifest.GeneratedFile{
					Path:     relPath,
					Checksum: checksum,
				})
			}
		}

		dep := manifest.ManifestDependency{
			Name:                 dependency.Name,
			TemplateURL:          dependencyOptions.TemplateURL,
			OutputFolder:         dependencyOptions.OutputFolder,
			SourceChecksum:       depResult.SourceChecksum,
			Skip:                 dependency.Skip,
			ForEach:              forEach,
			ForEachReference:     dependency.ForEachReference,
			VarFiles:             dependency.VarFiles,
			Variables:            resolvedVars,
			Files:                depFiles,
			DontInheritVariables: dependency.DontInheritVariables,
		}

		mu.Lock()
		allDeps = append(allDeps, dep)
		mu.Unlock()

		return nil
	}

	forEach := dependency.ForEach

	if len(dependency.ForEachReference) > 0 {
		renderedReference, renderErr := render.RenderTemplateFromStringWithContext(ctx, opts.TemplateFolder, dependency.ForEachReference, originalVars, opts)
		if renderErr != nil {
			return nil, renderErr
		}

		value, unmarshalErr := variables.UnmarshalListOfStrings(originalVars, renderedReference)
		if unmarshalErr != nil {
			return nil, unmarshalErr
		}

		forEach = value
	}

	if len(forEach) > 0 {
		g, ctx := errgroup.WithContext(ctx)
		if opts.Parallelism > 0 {
			g.SetLimit(opts.Parallelism)
		}

		for _, item := range forEach {
			g.Go(func() error {
				updatedVars := util.MergeMaps(originalVars, map[string]any{eachVarName: item})
				return doProcess(ctx, updatedVars, []string{item})
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
	} else {
		if processErr := doProcess(ctx, originalVars, nil); processErr != nil {
			return nil, processErr
		}
	}

	return allDeps, nil
}

// Clone the given options for use when rendering the given dependency. The dependency will get the same options as
// the original passed in, except for the template folder, output folder, and command-line vars.
func cloneOptionsForDependency(
	ctx context.Context,
	dependency *variables.Dependency,
	originalOpts *options.BoilerplateOptions,
	variablesInConfig map[string]variables.Variable,
	variables map[string]any,
) (*options.BoilerplateOptions, error) {
	renderedTemplateURL, err := render.RenderTemplateFromStringWithContext(ctx, originalOpts.TemplateFolder, dependency.TemplateURL, variables, originalOpts)
	if err != nil {
		return nil, err
	}

	renderedOutputFolder, err := render.RenderTemplateFromStringWithContext(ctx, originalOpts.TemplateFolder, dependency.OutputFolder, variables, originalOpts)
	if err != nil {
		return nil, err
	}

	templateURL, templateFolder, err := getterhelper.DetermineTemplateConfig(renderedTemplateURL)
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
		renderedVarFilePath, err := render.RenderTemplateFromStringWithContext(ctx, originalOpts.TemplateFolder, varFilePath, variables, originalOpts)
		if err != nil {
			return nil, err
		}

		renderedVarFiles = append(renderedVarFiles, renderedVarFilePath)
	}

	vars, err := cloneVariablesForDependency(ctx, originalOpts, dependency, variablesInConfig, variables, renderedVarFiles)
	if err != nil {
		return nil, err
	}

	return &options.BoilerplateOptions{
		Vars:                    vars,
		TemplateURL:             templateURL,
		TemplateFolder:          templateFolder,
		OutputFolder:            outputFolder,
		OnMissingKey:            originalOpts.OnMissingKey,
		OnMissingConfig:         originalOpts.OnMissingConfig,
		NonInteractive:          originalOpts.NonInteractive,
		NoHooks:                 originalOpts.NoHooks,
		NoShell:                 originalOpts.NoShell,
		DisableDependencyPrompt: originalOpts.DisableDependencyPrompt,
		Parallelism:             originalOpts.Parallelism,
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
	ctx context.Context,
	opts *options.BoilerplateOptions,
	dependency *variables.Dependency,
	variablesInConfig map[string]variables.Variable,
	originalVariables map[string]any,
	renderedVarFiles []string,
) (map[string]any, error) {
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
	newVariables := map[string]any{}

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
			renderedValue, err := render.RenderTemplateFromStringWithContext(ctx, opts.TemplateFolder, strValue, currentVariables, opts)
			if err != nil {
				return nil, err
			}

			varValue = renderedValue
		}

		newVariables[variable.Name()] = varValue
		// Update currentVariables to include the newly processed variable
		currentVariables = util.MergeMaps(currentVariables, map[string]any{
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
func shouldProcessDependency(
	ctx context.Context,
	dependency *variables.Dependency,
	opts *options.BoilerplateOptions,
	variables map[string]any,
) (bool, error) {
	shouldSkip, err := shouldSkipDependency(ctx, dependency, opts, variables)
	if err != nil {
		return false, err
	}

	if shouldSkip {
		return false, nil
	}

	if opts.NonInteractive || opts.DisableDependencyPrompt {
		return true, nil
	}

	return prompt.PromptUserForYesNo(fmt.Sprintf("Process dependency '%s'?", dependency.Name))
}

// Return true if the skip parameter of the given dependency evaluates to a "true" value
func shouldSkipDependency(ctx context.Context, dependency *variables.Dependency, opts *options.BoilerplateOptions, variables map[string]any) (bool, error) {
	if dependency.Skip == "" {
		return false, nil
	}

	rendered, err := render.RenderTemplateFromStringWithContext(ctx, opts.TemplateFolder, dependency.Skip, variables, opts)
	if err != nil {
		return false, err
	}

	logging.Logger.Printf("Skip attribute for dependency %s evaluated to '%s'", dependency.Name, rendered)

	return rendered == "true", nil
}

// processTemplateFolder copies all the files and folders in templateFolder to outputFolder, passing text files through the Go template engine
// with the given set of variables as the data. Returns the list of generated file paths relative to the output directory.
func processTemplateFolder(
	ctx context.Context,
	config *config.BoilerplateConfig,
	opts *options.BoilerplateOptions,
	variables map[string]any,
	partials []string,
) ([]string, error) {
	logging.Logger.Printf("Processing templates in %s and outputting generated files to %s", opts.TemplateFolder, opts.OutputFolder)

	// Process and render skip files and engines before walking so we only do the rendering operation once.
	processedSkipFiles, err := processSkipFiles(ctx, config.SkipFiles, opts, variables)
	if err != nil {
		return nil, err
	}

	processedEngines, err := processEngines(ctx, config.Engines, opts, variables)
	if err != nil {
		return nil, err
	}

	var generatedFilePaths []string

	walkErr := filepath.WalkDir(opts.TemplateFolder, func(path string, d fs.DirEntry, err error) error {
		path = filepath.ToSlash(path)

		switch {
		case shouldSkipPath(path, opts, processedSkipFiles):
			logging.Logger.Printf("Skipping %s", path)
			return nil
		case fileutil.IsDir(path):
			return createOutputDir(ctx, path, opts, variables)
		default:
			engine := determineTemplateEngine(processedEngines, path)

			filePath, processErr := processFile(ctx, path, opts, variables, partials, engine)
			if processErr != nil {
				return processErr
			}

			if filePath != "" {
				generatedFilePaths = append(generatedFilePaths, filePath)
			}

			return nil
		}
	})

	return generatedFilePaths, walkErr
}

// processFile copies the given path, which is in the folder templateFolder, to the outputFolder, passing it through the Go template
// engine with the given set of variables as the data if it's a text file. Returns the relative path of the generated file.
func processFile(
	ctx context.Context,
	path string,
	opts *options.BoilerplateOptions,
	variables map[string]any,
	partials []string,
	engine variables.TemplateEngineType,
) (string, error) {
	isText, err := fileutil.IsTextFile(path)
	if err != nil {
		return "", err
	}

	if isText {
		return processTemplate(ctx, path, opts, variables, partials, engine)
	} else {
		return copyFile(ctx, path, opts, variables)
	}
}

// Create the given directory, which is in templateFolder, in the given outputFolder
func createOutputDir(ctx context.Context, dir string, opts *options.BoilerplateOptions, variables map[string]any) error {
	destination, err := outPath(ctx, dir, opts, variables)
	if err != nil {
		return err
	}

	logging.Logger.Printf("Creating folder %s", destination)

	return os.MkdirAll(destination, defaultDirPerm)
}

// Compute the path where the given file, which is in templateFolder, should be copied in outputFolder. If the file
// path contains boilerplate syntax, use the given options and variables to render it to determine the final output
// path.
func outPath(ctx context.Context, file string, opts *options.BoilerplateOptions, variables map[string]any) (string, error) {
	templateFolderAbsPath, err := filepath.Abs(opts.TemplateFolder)
	if err != nil {
		return "", err
	}

	// | is an illegal filename char in Windows, so we also support urlencoded chars in the path. To support this, we
	// first urldecode the file before passing it through.
	urlDecodedFile, err := url.QueryUnescape(file)
	if err != nil {
		return "", err
	}

	interpolatedFilePath, err := render.RenderTemplateFromStringWithContext(ctx, file, urlDecodedFile, variables, opts)
	if err != nil {
		return "", err
	}

	fileAbsPath, err := filepath.Abs(interpolatedFilePath)
	if err != nil {
		return "", err
	}

	relPath, err := filepath.Rel(templateFolderAbsPath, fileAbsPath)
	if err != nil {
		return "", err
	}

	return path.Join(opts.OutputFolder, relPath), nil
}

// Copy the given file, which is in options.TemplateFolder, to options.OutputFolder.
// Returns the relative path of the copied file from the output directory.
func copyFile(ctx context.Context, file string, opts *options.BoilerplateOptions, variables map[string]any) (string, error) {
	destination, err := outPath(ctx, file, opts, variables)
	if err != nil {
		return "", err
	}

	logging.Logger.Printf("Copying %s to %s", file, destination)

	if err := fileutil.CopyFile(file, destination); err != nil {
		return "", err
	}

	relPath, err := filepath.Rel(opts.OutputFolder, destination)
	if err != nil {
		relPath = destination
	}

	return relPath, nil
}

// processTemplate runs the template at templatePath, which is in templateFolder, through the Go template engine with the given
// variables as data and writes the result to outputFolder. Returns the relative path of the generated file.
func processTemplate(
	ctx context.Context,
	templatePath string,
	opts *options.BoilerplateOptions,
	vars map[string]any,
	partials []string,
	engine variables.TemplateEngineType,
) (string, error) {
	destination, err := outPath(ctx, templatePath, opts, vars)
	if err != nil {
		return "", err
	}

	var out string

	switch engine {
	case variables.GoTemplate:
		out, err = render.RenderTemplateWithPartialsWithContext(ctx, templatePath, partials, vars, opts)
		if err != nil {
			return "", err
		}
	case variables.Jsonnet:
		out, err = render.RenderJsonnetTemplate(templatePath, vars, opts)
		if err != nil {
			return "", err
		}
		// Strip the jsonnet extension from the destination, if it exists.
		destination = strings.TrimSuffix(destination, ".jsonnet")
	}

	if err := fileutil.WriteFileWithSamePermissions(templatePath, destination, []byte(out)); err != nil {
		return "", err
	}

	relPath, err := filepath.Rel(opts.OutputFolder, destination)
	if err != nil {
		relPath = destination
	}

	return relPath, nil
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
