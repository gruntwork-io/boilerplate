//go:build js && wasm

// Package inputs exposes the boilerplateInputsMap js.Func factory. Importing
// this package pulls in config and its dependencies, so it is kept out of
// the lite build.
package inputs

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/inputs"
)

// Required argument count for boilerplateInputsMap.
const expectedArgs = 2

// Handler returns a js.Func that wraps inputs.FromFS. It is the WASM-side
// counterpart to `boilerplate inputs map`: it takes a templateBundle and a
// JSON vars object, runs the static analysis described in the inputs
// package, and returns the result as a JSON string. On error it returns an
// Error whose `kind` property holds a stable identifier — see the Kind*
// constants in cmd/wasm/internal/bundlewasm — so JS callers can switch on
// the failure mode without matching free-form messages. Remote dependency
// template-urls are not resolvable in WASM and produce
// "unresolvable_dependency" entries in the result's errors array.
//
// JS signature:
//
//	boilerplateInputsMap(bundleJSON: string, varsJSON: string) -> string | Error
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("inputsMap")

		if len(args) < expectedArgs {
			return bundlewasm.StructuralError("boilerplateInputsMap requires 2 arguments: bundleJSON, varsJSON")
		}

		bundle, err := bundlewasm.DecodeBundle(args[0].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		var variables map[string]any
		if err := json.Unmarshal([]byte(args[1].String()), &variables); err != nil {
			return bundlewasm.StructuralError(fmt.Sprintf("failed to parse variables JSON: %v", err))
		}

		result, err := inputs.FromFS(context.Background(), bundle.FS, bundle.RootPath, variables)
		if err != nil {
			return bundlewasm.RenderError(fmt.Sprintf("inputs analysis failed: %v", err))
		}

		out, err := json.Marshal(result)
		if err != nil {
			return bundlewasm.RenderError(fmt.Sprintf("failed to marshal result: %v", err))
		}

		return string(out)
	})
}
