//go:build js && wasm

// Package inputs exposes the boilerplateInputsMap js.Func factory. Importing
// this package pulls in config and its dependencies, so it is kept out of
// the lite build.
package inputs

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
// dependency tree.
type templateBundle struct {
	RootPath string            `json:"rootPath"`
	Files    map[string]string `json:"files"`
}

// Handler returns a js.Func that wraps inputs.FromFS. It is the WASM-side
// counterpart to `boilerplate inputs map`: it takes a templateBundle and a
// JSON vars object, runs the static analysis described in the inputs
// package, and returns the result as a JSON string. On error it returns an
// Error. Remote dependency template-urls are not resolvable in WASM and
// produce "unresolvable_dependency" entries in the result's errors array.
//
// JS signature:
//
//	boilerplateInputsMap(bundleJSON: string, varsJSON: string) -> string | Error
func Handler() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in inputsMap:", r)
			}
		}()

		if len(args) < 2 {
			return js.Global().Get("Error").New("boilerplateInputsMap requires 2 arguments: bundleJSON, varsJSON")
		}

		ctx := context.Background()
		bundleJSON := args[0].String()
		varsJSON := args[1].String()

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

		var variables map[string]any
		if err := json.Unmarshal([]byte(varsJSON), &variables); err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("failed to parse variables JSON: %v", err))
		}

		result, err := inputs.FromFS(ctx, mfs, rootPath, variables)
		if err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("inputs analysis failed: %v", err))
		}

		out, err := json.Marshal(result)
		if err != nil {
			return js.Global().Get("Error").New(fmt.Sprintf("failed to marshal result: %v", err))
		}

		return string(out)
	})
}

// validateBundlePath rejects paths that would muddle the analyzer's contract
// that every key in templateBundle.Files is a canonical, forward-slash,
// strictly-relative path anchored at the bundle root. Without this, two keys
// could refer to the same logical file (producing duplicate entries in
// Result.Files), or a key could appear to escape the bundle.
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
