//go:build js && wasm

// Package render exposes the boilerplateRenderTemplate js.Func factory. It is
// intentionally a sibling of cmd/wasm/inputs so the lite WASM build can
// import this package without pulling in config and its dependencies.
package render

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
)

// Handler returns a js.Func that wraps render.RenderTemplateFromString.
// Synchronous; suitable for callers that only need string rendering.
//
// JS signature:
//
//	boilerplateRenderTemplate(templateStr: string, varsJSON: string) -> string | Error
//
// On any failure returns a JS Error with `kind` set to the bundlewasm
// taxonomy (`structural` for arg-count / vars-JSON-parse failures,
// `render` for template-execution failures), so callers can switch on
// err.kind uniformly with the other WASM handlers.
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("renderTemplate")

		if len(args) < 2 {
			return bundlewasm.StructuralError("boilerplateRenderTemplate requires 2 arguments: templateStr, varsJSON")
		}

		templateStr := args[0].String()
		varsJSON := args[1].String()

		var variables map[string]any
		if err := json.Unmarshal([]byte(varsJSON), &variables); err != nil {
			return bundlewasm.StructuralError(fmt.Sprintf("failed to parse variables JSON: %v", err))
		}

		opts := &options.BoilerplateOptions{
			NonInteractive:  true,
			NoShell:         true,
			OnMissingKey:    options.ExitWithError,
			OnMissingConfig: options.Ignore,
		}

		result, err := render.RenderTemplateFromString(logging.Discard(), "template", templateStr, variables, opts)
		if err != nil {
			return bundlewasm.RenderError(fmt.Sprintf("template rendering failed: %v", err))
		}

		return result
	})
}
