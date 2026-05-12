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
      "sources": {
        "relative/path/to/file1": "<absolute_path_to_source_template_file>"
      },
      "errors": [
        { "kind": "undeclared_variable", "template": "...", "name": "...", "file": "..." }
      ]
    }

Keys in "inputs" are fully-qualified as <template_path>:<input_name>, where
<template_path> is "." for the root template and the dependency's
output-folder path (relative to the root output) for nested templates.

Keys in "sources" mirror those in "files": each output path maps to the
absolute filesystem path of the source template file that produces it.
Templates resolved via go-getter (git::, https://, ...) live under a temp
dir for the duration of the command; consumers needing the body should
read it before the process exits. Output paths whose filename template
failed to render are omitted from "sources" — the corresponding
"filename_render" entry in "errors" signals that the path is dynamic.

The command exits with a non-zero status only on unrecoverable parse failures.
Soft errors (e.g., a referenced variable that is not declared in any
boilerplate.yml in scope) appear in the "errors" array and do not change the
exit code.`

// newInputsCommand returns the "inputs" command with its "map" subcommand.
// It is exported only via CreateBoilerplateCli; defining it here keeps the
// subcommand wiring out of the main CLI file.
func newInputsCommand() *cli.Command {
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
					&cli.BoolFlag{
						Name:  options.OptIncludeBundle,
						Usage: "Include a `bundle` field in the JSON output containing the contents of every text file in the resolved dependency tree. The bundle is suitable for feeding back into the WASM `boilerplateInputsMap` and `boilerplateRenderFile` functions for warm-dispatch rendering.",
					},
				},
			},
		},
	}
}

// runInputsMap is the action handler for `boilerplate inputs map`. Tests
// inject stdout/stderr by setting app.Writer / app.ErrWriter before calling
// app.Run, which is why output routes through c.App rather than os.Stdout
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

func runInputsMapTo(c *cli.Context, stdout, stderr io.Writer) error {
	vars, err := variables.ParseVars(c.StringSlice(options.OptVar), c.StringSlice(options.OptVarFile))
	if err != nil {
		writeJSONError(stdout, inputs.KindParseArgs, err)
		return cli.Exit("", 1)
	}

	templateURL, templateFolder, err := getterhelper.DetermineTemplateConfig(c.String(options.OptTemplateURL))
	if err != nil {
		writeJSONError(stdout, inputs.KindParseArgs, err)
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

	ctx := context.Background()

	result, err := inputs.FromOptions(ctx, logger, opts)
	if err != nil {
		writeJSONError(stdout, inputs.KindParse, err)
		return cli.Exit("", 1)
	}

	// When --include-bundle is set, re-walk the resolved tree to collect
	// file contents and emit them in a new `bundle` field. The bundle
	// walk is independent of the analyzer pass: it would be cheaper to
	// do both in one traversal, but keeping them decoupled avoids
	// destabilizing the analyzer's existing contract (Result.Sources,
	// Errors composition, etc.). For typical templates the second walk
	// is negligible compared to go-getter download time.
	var bundle *inputs.Bundle

	if c.Bool(options.OptIncludeBundle) {
		// Use a fresh opts for the bundle walk: FromOptions has by now
		// populated opts.TemplateFolder with the resolved root (so
		// resolveRootLocation inside BundleFromOptions takes the fast
		// path for an already-local folder rather than re-running
		// go-getter).
		b, notes, bundleErr := inputs.BundleFromOptions(ctx, logger, opts)
		if bundleErr != nil {
			writeJSONError(stdout, inputs.KindParse, bundleErr)
			return cli.Exit("", 1)
		}

		bundle = b

		// Surface bundle notes via the existing errors[] array. They
		// share the same Kind vocabulary, so consumers don't need to
		// learn a second error shape.
		for _, n := range notes {
			result.Errors = append(result.Errors, inputs.AnalysisError{
				Kind:    n.Kind,
				Name:    n.Name,
				Message: n.Message,
			})
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")

	// When the bundle is requested, encode an envelope around result so
	// the existing fields remain at the top level (back-compat for any
	// consumer that doesn't read `bundle`). Without --include-bundle we
	// emit the bare result, preserving the byte-for-byte pre-change
	// output.
	if bundle != nil {
		envelope := struct {
			*inputs.Result
			Bundle *inputs.Bundle `json:"bundle"`
		}{
			Result: result,
			Bundle: bundle,
		}

		if encErr := enc.Encode(envelope); encErr != nil {
			return fmt.Errorf("encode result: %w", encErr)
		}

		return nil
	}

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

	if encErr := enc.Encode(doc); encErr != nil {
		_, _ = fmt.Fprintf(w, "{\"errors\":[{\"kind\":\"encode\",\"message\":%q}]}\n", encErr.Error())
	}
}
