//go:build js && wasm

package preparedbundle

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall/js"
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

// prepareHandler parses bundleJSON, validates every path the producer
// recorded, and stores the resulting PreparedBundle in store. Returns
// the handle ID on success, a structural JS Error on failure.
func prepareHandler(store *bundleStore) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in prepareBundle:", r)
			}
		}()

		if len(args) < 1 {
			return structuralError("boilerplatePrepareBundle requires 1 argument: bundleJSON")
		}

		bundleJSON := args[0].String()

		prepared, prepErr := parseBundle(bundleJSON)
		if prepErr != nil {
			return structuralError(prepErr.Error())
		}

		return store.Store(prepared)
	})
}

// renderFilesWithHandleHandler resolves the handle, parses the paths
// and vars arguments, and runs the prepared bundle's RenderFiles. The
// per-path error / classifier path is identical to
// cmd/wasm/renderfiles so the consumer's existing per-kind dispatch
// keeps working.
func renderFilesWithHandleHandler(store *bundleStore) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in renderFilesWithHandle:", r)
			}
		}()

		if len(args) < 3 {
			return structuralError("boilerplateRenderFilesWithHandle requires 3 arguments: handle, pathsJSON, varsJSON")
		}

		handle := args[0].String()
		pathsJSON := args[1].String()
		varsJSON := args[2].String()

		bundle := store.Get(handle)
		if bundle == nil {
			return structuralError(fmt.Sprintf("unknown or released bundle handle %q", handle))
		}

		var outputPaths []string
		if err := json.Unmarshal([]byte(pathsJSON), &outputPaths); err != nil {
			return structuralError(fmt.Sprintf("failed to parse outputPaths JSON: %v", err))
		}

		if len(outputPaths) == 0 {
			return structuralError("outputPaths must be a non-empty array")
		}

		var rawVars map[string]any
		if err := json.Unmarshal([]byte(varsJSON), &rawVars); err != nil {
			return structuralError(fmt.Sprintf("failed to parse variables JSON: %v", err))
		}

		vars := liftInputsToRoot(rawVars)

		raw := bundle.RenderFiles(context.Background(), outputPaths, vars)

		payload := resultPayload{Results: make([]perFileResult, 0, len(raw))}
		for _, r := range raw {
			if r.Err != nil {
				kind := classifyError(r.Err)
				payload.Results = append(payload.Results, perFileResult{
					Path: r.Path,
					Error: &perFileError{
						Kind:    kind,
						Message: fmt.Sprintf("%s: %v", kind, r.Err),
					},
				})

				continue
			}

			payload.Results = append(payload.Results, perFileResult{Path: r.Path, Content: r.Content})
		}

		out, err := json.Marshal(payload)
		if err != nil {
			return structuralError(fmt.Sprintf("failed to marshal results: %v", err))
		}

		return string(out)
	})
}

// releaseHandler removes the handle from the store. Idempotent (the
// store does the work; the handler is a thin wrapper).
func releaseHandler(store *bundleStore) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in releaseBundle:", r)
			}
		}()

		if len(args) < 1 {
			return js.Undefined()
		}

		store.Release(args[0].String())

		return js.Undefined()
	})
}

// structuralError builds the JS Error object for the structural path,
// matching the shape boilerplateRenderFiles uses. Same .kind taxonomy
// so callers can switch on .kind regardless of which entry point
// produced it.
func structuralError(msg string) js.Value {
	errVal := js.Global().Get("Error").New(msg)
	errVal.Set("kind", "structural")

	return errVal
}
