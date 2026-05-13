//go:build js && wasm

// Package renderfile exposes the boilerplateRenderFile js.Func factory.
// It is the WASM warm-dispatch counterpart to running `boilerplate template`
// against a single file: given a bundle, an output path, and the user's
// variable values, it walks the dep tree, builds the dep-scoped variable
// map, and renders only the source template that produces outputPath.
//
// Like cmd/wasm/inputs, this package pulls in config + its dependencies,
// so it is kept out of the lite WASM build.
package renderfile

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/inputs"
)

// Handler returns a js.Func that wraps inputs.RenderFileFromFS.
//
// JS signature:
//
//	boilerplateRenderFile(bundleJSON: string, outputPath: string, varsJSON: string) -> string | Error
//
// varsJSON is the same shape `boilerplate template --var-file` accepts —
// typically a top-level YAML/JSON object. For consumer parity with the
// runbooks contract, top-level "inputs" entries are lifted onto the root
// scope (so `{{ .Foo }}` works as well as `{{ .inputs.Foo }}`), while the
// "inputs" and "outputs" namespaces are preserved so deeper template
// expressions that reference them keep working.
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("renderFile")

		if len(args) < 3 {
			return js.Global().Get("Error").New("boilerplateRenderFile requires 3 arguments: bundleJSON, outputPath, varsJSON")
		}

		bundle, err := bundlewasm.DecodeBundle(args[0].String())
		if err != nil {
			return js.Global().Get("Error").New(err.Error())
		}

		outputPath := args[1].String()

		vars, err := bundlewasm.ParseAndLiftVars(args[2].String())
		if err != nil {
			return js.Global().Get("Error").New(err.Error())
		}

		rendered, err := inputs.RenderFileFromFS(context.Background(), bundle.FS, bundle.RootPath, outputPath, vars, bundle.Dependencies)
		if err != nil {
			// Tag sentinel errors with stable kind: prefixes so JS callers
			// can route specific failure modes to cold render without
			// matching free-form strings.
			kind := bundlewasm.ClassifyError(err)
			errVal := js.Global().Get("Error").New(fmt.Sprintf("%s: %v", kind, err))
			errVal.Set("kind", kind)

			return errVal
		}

		return rendered
	})
}
