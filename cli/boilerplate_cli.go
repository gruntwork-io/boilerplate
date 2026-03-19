// Package cli provides the command-line interface for the boilerplate tool.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/gruntwork-io/boilerplate/internal/manifest"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/templates"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/gruntwork-io/boilerplate/version"
)

const customHelpText = `Usage: {{.UsageText}}

A tool for generating files and folders (\"boilerplate\") from a set of templates. Examples:

Generate a project in ~/output from the templates in ~/templates:

    boilerplate --template-url ~/templates --output-folder ~/output

Generate a project in ~/output from the templates in ~/templates, using variables passed in via the command line:

    boilerplate --template-url ~/templates --output-folder ~/output --var "Title=Boilerplate" --var "ShowLogo=false"

Generate a project in ~/output from the templates in ~/templates, using variables read from a file:

    boilerplate --template-url ~/templates --output-folder ~/output --var-file vars.yml

Generate a project in ~/output from the templates in this repo's include example dir, using variables read from a file:

	boilerplate --template-url "git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/include?ref=main" --output-folder ~/output --var-file vars.yml


Options:

   {{range .VisibleFlags}}{{.}}
   {{end}}`

func CreateBoilerplateCli() *cli.App {
	cli.AppHelpTemplate = customHelpText
	app := cli.NewApp()

	app.Name = "boilerplate"
	app.Authors = []*cli.Author{
		{
			Name:  "Gruntwork",
			Email: "www.gruntwork.io",
		},
	}
	app.UsageText = "boilerplate [OPTIONS]"
	app.Version = version.GetVersion()
	app.Action = runApp

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  options.OptTemplateURL,
			Usage: "Generate the project from the templates in `URL`. This can be a local path, or a go-getter compatible URL for remote templates (e.g., `git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/include?ref=main`).",
		},
		&cli.StringFlag{
			Name:  options.OptOutputFolder,
			Usage: "Create the output files and folders in `FOLDER`.",
		},
		&cli.BoolFlag{
			Name:  options.OptNonInteractive,
			Usage: fmt.Sprintf("Do not prompt for input variables. All variables must be set via --%s and --%s options instead.", options.OptVar, options.OptVarFile),
		},
		&cli.StringSliceFlag{
			Name:  options.OptVar,
			Usage: "Use `NAME=VALUE` to set variable NAME to VALUE. May be specified more than once.",
		},
		&cli.StringSliceFlag{
			Name:  options.OptVarFile,
			Usage: "Load variable values from the YAML file `FILE`. May be specified more than once.",
		},
		&cli.StringFlag{
			Name:  options.OptMissingKeyAction,
			Usage: fmt.Sprintf("What `ACTION` to take if a template looks up a variable that is not defined. Must be one of: %s. Default: %s.", options.AllMissingKeyActions, options.DefaultMissingKeyAction),
		},
		&cli.StringFlag{
			Name:  options.OptMissingConfigAction,
			Usage: fmt.Sprintf("What `ACTION` to take if a the template folder does not contain a boilerplate.yml file. Must be one of: %s. Default: %s.", options.AllMissingConfigActions, options.DefaultMissingConfigAction),
		},
		&cli.BoolFlag{
			Name:  options.OptNoHooks,
			Usage: "If this flag is set, no hooks will execute.",
		},
		&cli.BoolFlag{
			Name:  options.OptNoShell,
			Usage: "If this flag is set, no shell helpers will execute. They will instead return the text 'replace-me'.",
		},
		&cli.BoolFlag{
			Name:  options.OptDisableDependencyPrompt,
			Usage: fmt.Sprintf("Do not prompt for confirmation to include dependencies. Has the same effect as --%s, without disabling variable prompts.", options.OptNonInteractive),
		},
		&cli.BoolFlag{
			Name:  options.OptManifest,
			Usage: "Write a manifest of all generated files (with checksums) to the output directory.",
		},
		&cli.StringFlag{
			Name:  options.OptManifestFile,
			Usage: "Write the manifest to `FILE` instead of the default location. Implies --manifest. Format is auto-detected from extension (.yaml/.yml for YAML, otherwise JSON).",
		},
	}

	// We pass JSON/YAML content to various CLI flags, such as --var, and this JSON/YAML content may contain commas or
	// other separators urfave/cli would treat as a slice separator, and would therefore break the value into multiple
	// parts in the middle of the JSON/YAML, which is not what we want. So here, we disable the slice separator to
	// avoid that issue. This means you have to pass --var multiple times to get multiple values, which is what we
	// want anyway. See https://github.com/urfave/cli/issues/1134 for more details.
	app.DisableSliceFlagSeparator = true

	return app
}

// When you run the CLI, this is the action function that gets called
func runApp(cliContext *cli.Context) error {
	if !cliContext.Args().Present() && cliContext.NumFlags() == 0 {
		return cli.ShowAppHelp(cliContext)
	}

	opts, err := ParseCLIContext(cliContext)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// The root boilerplate.yml is not itself a dependency, so we pass an empty Dependency.
	emptyDep := variables.Dependency{}

	result, err := templates.ProcessTemplateWithContext(ctx, opts, opts, &emptyDep)
	if err != nil {
		return err
	}

	if opts.Manifest {
		files, checksumErr := computeChecksums(opts.OutputFolder, result.GeneratedFiles)
		if checksumErr != nil {
			return checksumErr
		}

		m := manifest.NewManifest(opts.TemplateURL, opts.OutputFolder, result.SourceChecksum, files)

		manifestPath := filepath.Join(opts.OutputFolder, manifest.DefaultManifestFilename)
		if opts.ManifestFile != "" {
			manifestPath = opts.ManifestFile
		}

		if err := manifest.WriteManifest(manifestPath, m); err != nil {
			return err
		}
	}

	return nil
}

// computeChecksums streams each generated file through a SHA256 hasher.
func computeChecksums(outputDir string, relativePaths []string) ([]manifest.GeneratedFile, error) {
	files := make([]manifest.GeneratedFile, 0, len(relativePaths))

	for _, relPath := range relativePaths {
		absPath := filepath.Join(outputDir, relPath)

		checksum, err := sha256File(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to compute checksum for %s: %w", absPath, err)
		}

		files = append(files, manifest.GeneratedFile{
			Path:     relPath,
			Checksum: checksum,
		})
	}

	return files, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
