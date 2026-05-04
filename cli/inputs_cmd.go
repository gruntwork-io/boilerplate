package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/gruntwork-io/boilerplate/getterhelper"
	"github.com/gruntwork-io/boilerplate/inputs"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/variables"
)

const inputsMapHelpText = `Usage: boilerplate inputs map [OPTIONS]

Analyze the template at --template-url and print a JSON object describing which
output files are affected by each declared input variable. The command never
renders or writes any output files; it parses each template's AST to compute
the mapping.

The output schema is:

    {
      "inputs": {
        "<template_path>:<input_name>": {
          "name": "<input_name>",
          "declared_in": "<template_path>",
          "files": ["relative/path/to/file1", ...],
          "type": "string|bool|int|list|map|...",
          "description": "<from boilerplate.yml if present>"
        }
      },
      "files": {
        "relative/path/to/file1": ["<template_path>:<input_name>", ...]
      },
      "errors": [
        { "kind": "undeclared_variable", "template": "...", "name": "...", "file": "..." }
      ]
    }

Keys in "inputs" are fully-qualified as <template_path>:<input_name>, where
<template_path> is "." for the root template and the dependency's
output-folder path (relative to the root output) for nested templates.

The command exits with a non-zero status only on unrecoverable parse failures.
Soft errors (e.g., a referenced variable that is not declared in any
boilerplate.yml in scope) appear in the "errors" array and do not change the
exit code.`

// addInputsCommand returns the "inputs" command with its "map" subcommand.
// It is exported only via CreateBoilerplateCli; defining it here keeps the
// subcommand wiring out of the main CLI file.
func addInputsCommand() *cli.Command {
	return &cli.Command{
		Name:  "inputs",
		Usage: "Inspect declared input variables for a template.",
		Subcommands: []*cli.Command{
			{
				Name:        "map",
				Usage:       "Print a JSON map of declared inputs to the output files they affect.",
				Description: inputsMapHelpText,
				Action:      runInputsMap,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     options.OptTemplateURL,
						Usage:    "Generate the input mapping for the template at `URL`. Same resolution rules as `boilerplate template`.",
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:  options.OptVar,
						Usage: "Use `NAME=VALUE` to set variable NAME to VALUE. Used when rendering output filenames so the reported paths match what `boilerplate template` would produce. May be specified more than once.",
					},
					&cli.StringSliceFlag{
						Name:  options.OptVarFile,
						Usage: "Load variable values from the YAML file `FILE`. May be specified more than once.",
					},
				},
			},
		},
	}
}

// runInputsMap is the action handler for `boilerplate inputs map`.
//
// It builds a minimal BoilerplateOptions, runs inputs.FromOptions, and writes
// the JSON result to the cli app's Writer (stdout by default). On
// unrecoverable failures (e.g., the template cannot be downloaded or the root
// boilerplate.yml does not parse), it writes a JSON error object and exits
// non-zero.
//
// Tests can intercept the output by setting app.Writer / app.ErrWriter before
// calling app.Run; that is why we route through c.App rather than os.Stdout
// directly.
func runInputsMap(c *cli.Context) error {
	stdout := io.Writer(os.Stdout)
	if c.App != nil && c.App.Writer != nil {
		stdout = c.App.Writer
	}

	stderr := io.Writer(os.Stderr)
	if c.App != nil && c.App.ErrWriter != nil {
		stderr = c.App.ErrWriter
	}

	return runInputsMapTo(c, stdout, stderr)
}

// runInputsMapTo is the testable variant of runInputsMap with explicit
// stdout/stderr.
func runInputsMapTo(c *cli.Context, stdout, stderr io.Writer) error {
	vars, err := variables.ParseVars(c.StringSlice(options.OptVar), c.StringSlice(options.OptVarFile))
	if err != nil {
		writeJSONError(stdout, "parse_args", err)
		return cli.Exit("", 1)
	}

	templateURL, templateFolder, err := getterhelper.DetermineTemplateConfig(c.String(options.OptTemplateURL))
	if err != nil {
		writeJSONError(stdout, "parse_args", err)
		return cli.Exit("", 1)
	}

	opts := &options.BoilerplateOptions{
		Vars:            vars,
		TemplateURL:     templateURL,
		TemplateFolder:  templateFolder,
		NonInteractive:  true,
		NoHooks:         true,
		NoShell:         true,
		OnMissingKey:    options.ZeroValue,
		OnMissingConfig: options.Exit,
	}

	logger := logging.New(stderr, logging.LevelWarn)

	result, err := inputs.FromOptions(context.Background(), logger, opts)
	if err != nil {
		writeJSONError(stdout, "parse", err)
		return cli.Exit("", 1)
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	if encErr := enc.Encode(result); encErr != nil {
		return fmt.Errorf("encode result: %w", encErr)
	}

	return nil
}

// writeJSONError writes a small JSON document with a top-level "errors" array
// containing a single entry matching the AnalysisError shape. Used when the
// command cannot produce a meaningful Result (e.g., the root config did not
// parse).
func writeJSONError(w io.Writer, kind string, err error) {
	doc := struct {
		Errors []inputs.AnalysisError `json:"errors"`
	}{
		Errors: []inputs.AnalysisError{
			{Kind: kind, Message: err.Error()},
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
}
