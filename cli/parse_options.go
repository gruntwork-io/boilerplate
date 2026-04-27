package cli

import (
	"github.com/urfave/cli/v2"

	"github.com/gruntwork-io/boilerplate/getterhelper"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
	"github.com/gruntwork-io/boilerplate/variables"
)

// ParseCLIContext parses the command line context provided by the user and returns the BoilerplateOptions struct.
// The given filesystem is used for any filesystem reads required to construct the options (currently only var-files).
func ParseCLIContext(cliContext *cli.Context, fsys vfs.FS) (*options.BoilerplateOptions, error) {
	vars, err := variables.ParseVars(fsys, cliContext.StringSlice(options.OptVar), cliContext.StringSlice(options.OptVarFile))
	if err != nil {
		return nil, err
	}

	missingKeyAction := options.DefaultMissingKeyAction
	missingKeyActionValue := cliContext.String(options.OptMissingKeyAction)

	if missingKeyActionValue != "" {
		missingKeyAction, err = options.ParseMissingKeyAction(missingKeyActionValue)
		if err != nil {
			return nil, err
		}
	}

	missingConfigAction := options.DefaultMissingConfigAction
	missingConfigActionValue := cliContext.String(options.OptMissingConfigAction)

	if missingConfigActionValue != "" {
		missingConfigAction, err = options.ParseMissingConfigAction(missingConfigActionValue)
		if err != nil {
			return nil, err
		}
	}

	templateURL, templateFolder, err := getterhelper.DetermineTemplateConfig(cliContext.String(options.OptTemplateURL))
	if err != nil {
		return nil, err
	}

	opts := &options.BoilerplateOptions{
		Vars:                    vars,
		ShellCommandAnswers:     make(map[string]bool),
		TemplateURL:             templateURL,
		TemplateFolder:          templateFolder,
		OutputFolder:            cliContext.String(options.OptOutputFolder),
		OnMissingKey:            missingKeyAction,
		OnMissingConfig:         missingConfigAction,
		NonInteractive:          cliContext.Bool(options.OptNonInteractive),
		NoHooks:                 cliContext.Bool(options.OptNoHooks),
		NoShell:                 cliContext.Bool(options.OptNoShell),
		DisableDependencyPrompt: cliContext.Bool(options.OptDisableDependencyPrompt),
		ExecuteAllShellCommands: false,
		Manifest:                cliContext.Bool(options.OptManifest) || cliContext.String(options.OptManifestFile) != "",
		ManifestFile:            cliContext.String(options.OptManifestFile),
		Parallelism:             cliContext.Int(options.OptParallelism),
	}

	if err := validateOptions(opts); err != nil {
		return nil, err
	}

	return opts, nil
}

// validateOptions checks that the options have reasonable values and returns an error if they don't.
func validateOptions(opts *options.BoilerplateOptions) error {
	if opts.TemplateURL == "" {
		return options.ErrTemplateURLOptionCannotBeEmpty
	}

	if err := getterhelper.ValidateTemplateURL(opts.TemplateURL); err != nil {
		return err
	}

	if opts.OutputFolder == "" {
		return options.ErrOutputFolderOptionCannotBeEmpty
	}

	return nil
}
