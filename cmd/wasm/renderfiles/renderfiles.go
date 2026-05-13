//go:build js && wasm

// Package renderfiles exposes the boilerplateRenderFiles js.Func factory.
// It is the bulk counterpart to cmd/wasm/renderfile: same bundle and same
// vars, but N output paths rendered in one call. The single-template lite
// build does not include this; it is registered only by cmd/wasm/full.
//
// The win over an N-call loop on the JS side is that bundle JSON parse,
// fstest.MapFS construction, and vars parse all happen once per call,
// regardless of how many paths the caller is rendering. Each path still
// triggers its own dep-tree walk inside inputs.RenderFilesFromFS — which
// is fast Go-side work — but the per-call JSON / MapFS overhead, which
// the runbooks team measured as the dominant warm-dispatch cost, is
// charged once.
package renderfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/inputs"
)

// Handler returns a js.Func that renders multiple output paths from a
// single bundle in one call.
//
// JS signature:
//
//	boilerplateRenderFiles(
//	    bundleJSON: string,
//	    outputPathsJSON: string,   // JSON-encoded non-empty string[]
//	    varsJSON: string,
//	) -> string | Error
//
// On structural failure (arg count, unparsable bundle/paths/vars JSON,
// invalid bundle path, empty paths array) returns a JS Error with
// .kind === "structural". On any non-structural outcome returns a
// JSON-encoded string of bundlewasm.ResultPayload — including the
// all-paths-failed case. Per-path errors live inside results[i].error
// with the same .kind taxonomy used by boilerplateRenderFile.
const expectedArgs = 3

func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("renderFiles")

		if len(args) < expectedArgs {
			return bundlewasm.StructuralError("boilerplateRenderFiles requires 3 arguments: bundleJSON, outputPathsJSON, varsJSON")
		}

		bundle, err := bundlewasm.DecodeBundle(args[0].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		outputPaths, err := bundlewasm.ParseOutputPaths(args[1].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		vars, err := bundlewasm.ParseAndLiftVars(args[2].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		raw := inputs.RenderFilesFromFS(context.Background(), bundle.FS, bundle.RootPath, outputPaths, vars, bundle.Dependencies)
		payload := bundlewasm.BuildResultPayload(raw)

		out, err := json.Marshal(payload)
		if err != nil {
			return bundlewasm.StructuralError(fmt.Sprintf("failed to marshal results: %v", err))
		}

		return string(out)
	})
}
