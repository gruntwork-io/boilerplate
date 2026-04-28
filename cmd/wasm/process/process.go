//go:build js && wasm

// Package process exposes the boilerplateProcessTemplate js.Func factory.
// Importing this package pulls in templates, dependencies, and config —
// so it is kept out of the lite build.
package process

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/bridge"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/templates"
)

// Handler returns a js.Func that runs templates.ProcessTemplateWithContext
// from JS.
//
// Returns a Promise because fs syscalls under GOOS=js are async — doing the
// work inline in a FuncOf callback would deadlock the Go runtime.
//
// Defaults diverge from the CLI deliberately: nonInteractive, noShell, and
// disableDependencyPrompt all default to true (interactive/shell paths would
// deadlock or fail under WASM), and onMissingConfig defaults to "ignore" so
// plain template folders without a boilerplate.yml still work.
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 1 {
			return rejectedPromise(js.Global().Get("Error").New("boilerplateProcessTemplate requires 1 argument: optionsJSON"))
		}

		reqJSON := args[0].String()

		return newPromise(func(resolve, reject js.Value) {
			defer func() {
				if r := recover(); r != nil {
					resolve.Invoke(responseToJS(bridge.ErrorResponse(fmt.Errorf("panic: %v", r), config.DrainValidationWarnings())))
				}
			}()

			_ = config.DrainValidationWarnings()

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
	})
}

// newPromise runs body in a goroutine so fs/network syscalls don't deadlock
// the syscall/js event loop.
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

// runProcessTemplate passes opts as both options and rootOpts because the
// WASM entry point is always a top-level run, and thisDep is nil for the
// same reason.
func runProcessTemplate(opts *options.BoilerplateOptions) (result *templates.ProcessResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during ProcessTemplate: %v", r)
		}
	}()

	return templates.ProcessTemplateWithContext(context.Background(), logging.Discard(), opts, opts, nil)
}

// responseToJS rebuilds slices as []any because js.ValueOf rejects []string
// and would otherwise produce opaque values on the JS side.
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
