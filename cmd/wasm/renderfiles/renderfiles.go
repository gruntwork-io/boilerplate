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
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"syscall/js"
	"testing/fstest"

	"github.com/gruntwork-io/boilerplate/inputs"
)

// templateBundle mirrors the shape accepted by cmd/wasm/renderfile.
// Duplicating the struct (rather than importing the other package's) keeps
// the WASM packages independent so cmd/wasm/lite doesn't transitively
// pull in the full bundle.
type templateBundle struct {
	RootPath     string                          `json:"rootPath"`
	Files        map[string]string               `json:"files"`
	Dependencies map[string][]inputs.ResolvedDep `json:"dependencies"`
}

// perFileError is the inline-error shape encoded in the JSON result for a
// single path that failed to render. Mirrors the .kind/.message contract
// that cmd/wasm/renderfile attaches to its Error objects, so callers can
// switch on .kind regardless of whether they're using the single-path or
// bulk entry point.
type perFileError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// perFileResult is one entry in the result array. Exactly one of Content
// or Error is set — but content of an empty file is a valid success, so
// we use a pointer for Error and a plain string for Content (encoding/json
// emits an empty string field on success, which is the documented shape).
type perFileResult struct {
	Path    string        `json:"path"`
	Content string        `json:"content,omitempty"`
	Error   *perFileError `json:"error,omitempty"`
}

// resultPayload is the top-level success shape: always {"results": [...]}.
type resultPayload struct {
	Results []perFileResult `json:"results"`
}

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
// On structural failure (arg count, unparseable bundle/paths/vars JSON,
// invalid bundle path, empty paths array) returns a JS Error with
// .kind === "structural". On any non-structural outcome returns a
// JSON-encoded string of resultPayload — including the all-paths-failed
// case. Per-path errors live inside results[i].error with the same .kind
// taxonomy used by boilerplateRenderFile, so the consumer's existing
// per-kind dispatch keeps working.
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in renderFiles:", r)
			}
		}()

		if len(args) < 3 {
			return structuralError("boilerplateRenderFiles requires 3 arguments: bundleJSON, outputPathsJSON, varsJSON")
		}

		ctx := context.Background()
		bundleJSON := args[0].String()
		outputPathsJSON := args[1].String()
		varsJSON := args[2].String()

		var bundle templateBundle
		if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
			return structuralError(fmt.Sprintf("failed to parse bundle JSON: %v", err))
		}

		rootPath := bundle.RootPath
		if rootPath == "" {
			rootPath = "."
		}

		if rootPath != "." {
			if err := validateBundlePath(rootPath); err != nil {
				return structuralError(fmt.Sprintf("invalid rootPath %q: %v", rootPath, err))
			}
		}

		mfs := fstest.MapFS{}
		for p, contents := range bundle.Files {
			if err := validateBundlePath(p); err != nil {
				return structuralError(fmt.Sprintf("invalid bundle path %q: %v", p, err))
			}

			mfs[p] = &fstest.MapFile{Data: []byte(contents)}
		}

		// Validate every BundlePath the producer recorded — same gate
		// renderfile applies. A producer that emits malformed bundle
		// paths is broken; reject the bundle up-front rather than letting
		// bad paths bleed into per-path renders.
		for parent, siblings := range bundle.Dependencies {
			if parent != "." {
				if err := validateBundlePath(parent); err != nil {
					return structuralError(fmt.Sprintf("invalid dependencies parent key %q: %v", parent, err))
				}
			}

			for _, dep := range siblings {
				if err := validateBundlePath(dep.BundlePath); err != nil {
					return structuralError(fmt.Sprintf("invalid bundle path %q for dependency %q under %q: %v", dep.BundlePath, dep.Name, parent, err))
				}
			}
		}

		var outputPaths []string
		if err := json.Unmarshal([]byte(outputPathsJSON), &outputPaths); err != nil {
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

		raw := inputs.RenderFilesFromFS(ctx, mfs, rootPath, outputPaths, vars, bundle.Dependencies)

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
			// Marshal of a struct with string fields shouldn't fail in
			// practice; if it does, surface it as structural so the
			// caller can route the whole batch to cold.
			return structuralError(fmt.Sprintf("failed to marshal results: %v", err))
		}

		return string(out)
	})
}

// structuralError builds the JS Error object for the structural path: a
// failure shape that's not encodable inline because no per-path render
// was attempted (or the bundle itself is unusable). Caller treats this
// as "route the whole batch to cold render".
func structuralError(msg string) js.Value {
	errVal := js.Global().Get("Error").New(msg)
	errVal.Set("kind", "structural")

	return errVal
}

// liftInputsToRoot mirrors the renderfile package's helper of the same
// name. Same contract: hoist top-level "inputs" entries onto the root
// scope so `{{ .Foo }}` and `{{ .inputs.Foo }}` both work; preserve the
// "inputs" and "outputs" namespaces; explicit root-scope keys win over
// lifted ones (matching variables-package precedence).
//
// Duplicated rather than imported to keep cmd/wasm/renderfile and
// cmd/wasm/renderfiles independent of each other's internals.
func liftInputsToRoot(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}

	out := make(map[string]any, len(raw)*2)
	for k, v := range raw {
		out[k] = v
	}

	inputsBlock, ok := raw["inputs"].(map[string]any)
	if !ok {
		return out
	}

	for k, v := range inputsBlock {
		if _, exists := out[k]; exists {
			continue
		}

		out[k] = v
	}

	return out
}

// classifyError maps an error from inputs.RenderFileFromFS (returned via
// RenderFilesFromFS) to the short kind string the JS caller switches on.
// Same taxonomy boilerplateRenderFile uses; duplicated here so the two
// packages stay independent.
func classifyError(err error) string {
	switch {
	case errors.Is(err, inputs.ErrOutputNotProduced):
		return "output_not_produced"
	case errors.Is(err, inputs.ErrDependencyNotInBundle):
		return "dependency_not_in_bundle"
	case errors.Is(err, inputs.ErrDynamicFilename):
		return "dynamic_filename"
	case errors.Is(err, inputs.ErrSkipFilesExcluded):
		return "skip_files_excluded"
	default:
		return "render"
	}
}

// validateBundlePath duplicates the rule applied by cmd/wasm/inputs and
// cmd/wasm/renderfile: every entry in templateBundle.Files must be a
// canonical, forward-slash, strictly-relative path anchored at the bundle
// root. The duplication keeps the three packages independent so a
// future build configuration that wants any one of them in isolation
// (e.g., a "renderfiles-only" lite-equivalent build) can drop the others.
func validateBundlePath(p string) error {
	if p == "" {
		return errors.New("empty path")
	}

	if strings.HasPrefix(p, "/") {
		return errors.New("absolute paths not allowed")
	}

	if strings.ContainsRune(p, '\\') {
		return errors.New("use forward slashes")
	}

	cleaned := path.Clean(p)
	if cleaned != p {
		return fmt.Errorf("non-canonical path; clean to %q", cleaned)
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return errors.New("path escapes bundle root")
	}

	return nil
}
