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

// templateBundle is the JSON shape accepted by Handler. Files is keyed by
// path relative to RootPath and must include every boilerplate.yml in the
// dependency tree, plus every template file. Dependencies is the
// pre-computed dep layout the producer wrote into the bundle: a missing or
// nil map signals an older bundle and the consumer fails with
// `dependency_not_in_bundle` so the caller routes to cold render. It
// mirrors the shape produced by `boilerplate inputs map --include-bundle`
// and the shape accepted by boilerplateInputsMap.
type templateBundle struct {
	RootPath     string                          `json:"rootPath"`
	Files        map[string]string               `json:"files"`
	Dependencies map[string][]inputs.ResolvedDep `json:"dependencies"`
}

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
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in renderFile:", r)
			}
		}()

		if len(args) < 3 {
			return js.Global().Get("Error").New("boilerplateRenderFile requires 3 arguments: bundleJSON, outputPath, varsJSON")
		}

		ctx := context.Background()
		bundleJSON := args[0].String()
		outputPath := args[1].String()
		varsJSON := args[2].String()

		var bundle templateBundle
		if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("failed to parse bundle JSON: %v", err))
		}

		rootPath := bundle.RootPath
		if rootPath == "" {
			rootPath = "."
		}

		if rootPath != "." {
			if err := validateBundlePath(rootPath); err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("invalid rootPath %q: %v", rootPath, err))
			}
		}

		mfs := fstest.MapFS{}
		for p, contents := range bundle.Files {
			if err := validateBundlePath(p); err != nil {
				return js.Global().Get("Error").New(fmt.Sprintf("invalid bundle path %q: %v", p, err))
			}

			mfs[p] = &fstest.MapFile{Data: []byte(contents)}
		}

		// Validate every BundlePath the producer recorded. A producer
		// that emits malformed bundle paths is broken; reject the bundle
		// up-front rather than letting bad paths leak into the renderer.
		for parent, siblings := range bundle.Dependencies {
			if parent != "." {
				if err := validateBundlePath(parent); err != nil {
					return js.Global().Get("Error").New(fmt.Sprintf("invalid dependencies parent key %q: %v", parent, err))
				}
			}

			for _, dep := range siblings {
				if err := validateBundlePath(dep.BundlePath); err != nil {
					return js.Global().Get("Error").New(fmt.Sprintf("invalid bundle path %q for dependency %q under %q: %v", dep.BundlePath, dep.Name, parent, err))
				}
			}
		}

		var rawVars map[string]any
		if err := json.Unmarshal([]byte(varsJSON), &rawVars); err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("failed to parse variables JSON: %v", err))
		}

		vars := liftInputsToRoot(rawVars)

		rendered, err := inputs.RenderFileFromFS(ctx, mfs, rootPath, outputPath, vars, bundle.Dependencies)
		if err != nil {
			// Tag sentinel errors with stable kind: prefixes so JS callers
			// can route specific failure modes to cold render without
			// matching free-form strings. The wrapped error remains the
			// human-readable message.
			kind := classifyError(err)
			msg := fmt.Sprintf("%s: %v", kind, err)
			errVal := js.Global().Get("Error").New(msg)
			errVal.Set("kind", kind)

			return errVal
		}

		return rendered
	})
}

// liftInputsToRoot returns a new map that hoists every key under the
// top-level "inputs" entry onto the root scope, while also preserving the
// original "inputs" and "outputs" namespaces. Same shape contract the
// runbooks consumer follows for cold render: `{{ .Foo }}` (legacy) and
// `{{ .inputs.Foo }}` both reference the same value.
//
// If "inputs" isn't a map, or isn't present, the original map is returned
// unchanged. We do NOT overwrite root keys that already exist — explicit
// root-scope entries take precedence (consistent with the variables
// package's CLI-flag-over-var-file ordering).
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

// classifyError maps an error from inputs.RenderFileFromFS to a short kind
// string the JS caller can switch on. Falls back to "render" for any error
// that isn't one of the sentinel cases.
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

// validateBundlePath duplicates the rule applied by cmd/wasm/inputs:
// every entry in templateBundle.Files must be a canonical,
// forward-slash, strictly-relative path anchored at the bundle root. The
// duplication is intentional — keeping the two packages independent
// avoids forcing cmd/wasm/lite to pull in renderfile transitively.
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
