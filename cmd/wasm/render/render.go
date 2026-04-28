//go:build js && wasm

// Package render exposes the boilerplateRenderTemplate js.Func factory. It is
// intentionally a sibling of cmd/wasm/process so the lite WASM build can
// import this package without pulling in templates/dependencies/config.
package render

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
)

// Handler returns a js.Func that wraps render.RenderTemplateFromString.
// Synchronous; suitable for callers that only need string rendering.
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
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

		result, err := render.RenderTemplateFromString(logging.Discard(), "template", templateStr, variables, opts)
		if err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("template rendering failed: %v", err))
		}

		return result
	})
}
