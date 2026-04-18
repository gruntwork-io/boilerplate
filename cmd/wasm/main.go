//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/bridge"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/render"
	"github.com/gruntwork-io/boilerplate/templates"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", js.FuncOf(renderTemplate))
	js.Global().Set("boilerplateProcessTemplate", js.FuncOf(processTemplate))

	// Block forever to keep Go runtime alive.
	select {}
}

func renderTemplate(this js.Value, args []js.Value) any {
	defer func() {
		if r := recover(); r != nil {
			// Panic recovery is best-effort; the JS caller will see undefined.
			fmt.Println("boilerplate: recovered from panic:", r)
		}
	}()

	if len(args) < 2 {
		return js.Global().Get("Error").New("boilerplateRenderTemplate requires 2 arguments: templateStr, varsJSON")
	}

	templateStr := args[0].String()
	varsJSON := args[1].String()

	var variables map[string]any
	if err := json.Unmarshal([]byte(varsJSON), &variables); err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("failed to parse variables JSON: %v", err))
	}

	opts := &options.BoilerplateOptions{
		NonInteractive:  true,
		NoShell:         true,
		OnMissingKey:    options.ExitWithError,
		OnMissingConfig: options.Ignore,
	}

	result, err := render.RenderTemplateFromString("template", templateStr, variables, opts)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("template rendering failed: %v", err))
	}

	return result
}

// processTemplate runs the full templates.ProcessTemplateWithContext pipeline
// so that callers get feature parity with the CLI (directory walk, skip_files,
// dependencies, partials, path-name templating, manifest, etc.) without the
// cost of spawning a subprocess.
//
// JS signature:
//
//	boilerplateProcessTemplate(optionsJSON: string) => Promise<{
//	  error: string,            // empty on success, otherwise the failure message
//	  generatedFiles: string[], // paths to files written by this run
//	  sourceChecksum: string,   // populated only when manifest=true
//	}>
//
// The function returns a Promise because Go's fs syscalls on GOOS=js are
// asynchronous: the Go runtime dispatches fs work to JS callbacks and awaits
// them on a channel, which would deadlock if we tried to complete the whole
// pipeline inside the synchronous FuncOf invocation. The caller must await
// the result.
//
// optionsJSON is a JSON-encoded object with the following fields. Anything
// not listed uses the default shown in parens:
//
//	templateFolder          string                   (required) local path to the template
//	outputFolder            string                   (required) local path to write into
//	vars                    object                   (optional) variable name -> value
//	varFiles                string[]                 (optional) paths to YAML var files
//	nonInteractive          bool                     (true)
//	noShell                 bool                     (true)  -- hooks are blocked
//	disableDependencyPrompt bool                     (true)
//	onMissingKey            "invalid"|"zero"|"error" ("error")
//	onMissingConfig         "exit"|"ignore"          ("ignore")
//	manifest                bool                     (false)
//
// Defaults that diverge from the CLI — these are deliberate for WASM and
// should not be "fixed" to match cli/parse_options.go:
//
//   - nonInteractive=true (CLI default: false). Interactive prompts call
//     syscall/js-hostile code (survey/tty); in WASM they would deadlock the
//     Go runtime since the JS event loop is blocked on our FuncOf callback.
//   - noShell=true (CLI default: false). There is no host shell under
//     GOOS=js, and running hooks would fail noisily. Block them up front.
//   - disableDependencyPrompt=true (CLI default: false). Dependency confirm
//     prompts are interactive — same deadlock risk as above.
//   - onMissingConfig="ignore" (CLI default: "exit"). WASM callers frequently
//     invoke boilerplate against plain template folders that lack a
//     boilerplate.yml. Failing hard would break the common case.
//
// Var-file precedence also matches the CLI: values from varFiles override
// inline vars on key conflict (see variables/yaml_helpers.go ParseVars).
//
// Argument-shape errors (wrong arity, bad JSON) reject the Promise with a JS
// Error, matching the convention used by boilerplateRenderTemplate. Render
// failures resolve the Promise with a response whose `error` field is set so
// callers can branch on a field instead of wrapping the call in try/catch.
func processTemplate(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return rejectedPromise(js.Global().Get("Error").New("boilerplateProcessTemplate requires 1 argument: optionsJSON"))
	}

	reqJSON := args[0].String()

	// Drain any warnings left over from a previous run before we start, so
	// whatever we collect during this invocation is scoped to this invocation.
	_ = config.DrainValidationWarnings()

	return newPromise(func(resolve, reject js.Value) {
		defer func() {
			if r := recover(); r != nil {
				resolve.Invoke(responseToJS(bridge.ErrorResponse(fmt.Errorf("panic: %v", r), config.DrainValidationWarnings())))
			}
		}()

		req, err := bridge.ParseProcessTemplateRequest(reqJSON)
		if err != nil {
			reject.Invoke(js.Global().Get("Error").New(err.Error()))
			return
		}

		opts, err := bridge.BuildBoilerplateOptions(req)
		if err != nil {
			resolve.Invoke(responseToJS(bridge.ErrorResponse(err, config.DrainValidationWarnings())))
			return
		}

		result, processErr := runProcessTemplate(opts)
		warnings := config.DrainValidationWarnings()
		if processErr != nil {
			resolve.Invoke(responseToJS(bridge.ErrorResponse(processErr, warnings)))
			return
		}

		resolve.Invoke(responseToJS(bridge.SuccessResponse(result.GeneratedFiles, result.SourceChecksum, warnings)))
	})
}

// newPromise constructs a JS Promise whose executor spawns a goroutine so the
// body runs off the syscall/js event loop. Filesystem and network syscalls
// under GOOS=js are asynchronous in the Go runtime, so doing them inline in a
// FuncOf callback would deadlock: the JS event loop is blocked on our
// callback while Go is blocked awaiting a JS callback.
func newPromise(body func(resolve, reject js.Value)) js.Value {
	var executor js.Func
	executor = js.FuncOf(func(_ js.Value, args []js.Value) any {
		resolve := args[0]
		reject := args[1]
		go func() {
			defer executor.Release()
			body(resolve, reject)
		}()
		return nil
	})

	return js.Global().Get("Promise").New(executor)
}

func rejectedPromise(reason js.Value) js.Value {
	return js.Global().Get("Promise").Call("reject", reason)
}

// runProcessTemplate invokes the process pipeline for the WASM entry point.
//
// Passes opts as both `options` and `rootOpts` because the WASM entry point
// is always a top-level run — there is no outer template invoking us as a
// dependency. Nested dependency processing is handled internally by
// ProcessTemplateWithContext, which re-enters itself with its own rootOpts
// for each dependency it discovers. thisDep is nil for the same reason: the
// top-level run is not itself a dependency of anything.
func runProcessTemplate(opts *options.BoilerplateOptions) (result *templates.ProcessResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during ProcessTemplate: %v", r)
		}
	}()

	return templates.ProcessTemplateWithContext(context.Background(), opts, opts, nil)
}

// responseToJS converts a ProcessTemplateResponse into a plain JS object. We
// build the map explicitly so that []string becomes a JS array and not a Go
// syscall/js opaque value.
func responseToJS(resp *bridge.ProcessTemplateResponse) any {
	generated := make([]any, len(resp.GeneratedFiles))
	for i, p := range resp.GeneratedFiles {
		generated[i] = p
	}

	warnings := make([]any, len(resp.Warnings))
	for i, w := range resp.Warnings {
		warnings[i] = w
	}

	return map[string]any{
		"error":          resp.Error,
		"generatedFiles": generated,
		"sourceChecksum": resp.SourceChecksum,
		"warnings":       warnings,
	}
}
