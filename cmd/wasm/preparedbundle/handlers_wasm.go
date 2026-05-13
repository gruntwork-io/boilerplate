//go:build js && wasm

package preparedbundle

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
)

// Handlers is the trio of js.Funcs the full WASM build registers as
// boilerplatePrepareBundle, boilerplateRenderFilesWithHandle, and
// boilerplateReleaseBundle. All three share a single store so that a
// handle returned by Prepare resolves correctly inside the matching
// Render or Release call.
type Handlers struct {
	Prepare               js.Func
	RenderFilesWithHandle js.Func
	Release               js.Func
}

// New wires up a fresh handle store and returns the three handlers
// that read from it. Production code calls this exactly once at main()
// time; the lifetime of the returned Handlers matches the lifetime of
// the Go runtime under WASM.
func New() Handlers {
	store := newBundleStore()

	return Handlers{
		Prepare:               prepareHandler(store),
		RenderFilesWithHandle: renderFilesWithHandleHandler(store),
		Release:               releaseHandler(store),
	}
}

func prepareHandler(store *bundleStore) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("prepareBundle")

		if len(args) < 1 {
			return bundlewasm.StructuralError("boilerplatePrepareBundle requires 1 argument: bundleJSON")
		}

		prepared, err := parseBundle(args[0].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		return store.Store(prepared)
	})
}

func renderFilesWithHandleHandler(store *bundleStore) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("renderFilesWithHandle")

		if len(args) < 3 {
			return bundlewasm.StructuralError("boilerplateRenderFilesWithHandle requires 3 arguments: handle, pathsJSON, varsJSON")
		}

		handle := args[0].String()

		bundle := store.Get(handle)
		if bundle == nil {
			return bundlewasm.StructuralError(fmt.Sprintf("unknown or released bundle handle %q", handle))
		}

		outputPaths, err := bundlewasm.ParseOutputPaths(args[1].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		vars, err := bundlewasm.ParseAndLiftVars(args[2].String())
		if err != nil {
			return bundlewasm.StructuralError(err.Error())
		}

		raw := bundle.RenderFiles(context.Background(), outputPaths, vars)
		payload := bundlewasm.BuildResultPayload(raw)

		out, err := json.Marshal(payload)
		if err != nil {
			return bundlewasm.StructuralError(fmt.Sprintf("failed to marshal results: %v", err))
		}

		return string(out)
	})
}

func releaseHandler(store *bundleStore) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer bundlewasm.RecoverPanic("releaseBundle")

		if len(args) < 1 {
			return js.Undefined()
		}

		store.Release(args[0].String())

		return js.Undefined()
	})
}
