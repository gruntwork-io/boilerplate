//go:build js && wasm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall/js"
	"testing/fstest"

	"github.com/gruntwork-io/boilerplate/inputs"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/render"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", js.FuncOf(renderTemplate))
	js.Global().Set("boilerplateInputsMap", js.FuncOf(inputsMap))

	// Block forever to keep Go runtime alive.
	select {}
}

func renderTemplate(this js.Value, args []js.Value) any {
	defer func() {
		if r := recover(); r != nil {
			// Panic recovery is best-effort; the JS caller will see undefined.
			fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic:", r)
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
}

// templateBundle is the JSON shape accepted by inputsMap. Files is keyed by
// path relative to RootPath and must include every boilerplate.yml in the
// dependency tree.
type templateBundle struct {
	RootPath string            `json:"rootPath"`
	Files    map[string]string `json:"files"`
}

// inputsMap is the WASM-side counterpart to `boilerplate inputs map`. It takes
// a templateBundle and a JSON vars object, runs the static analysis described
// in the inputs package, and returns the result as a JSON string. On error it
// returns an Error. Remote dependency template-urls are not resolvable in WASM
// and produce "unresolvable_dependency" entries in the result's errors array.
//
// JS signature:
//
//	boilerplateInputsMap(bundleJSON: string, varsJSON: string) -> string | Error
func inputsMap(this js.Value, args []js.Value) any {
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

	mfs := fstest.MapFS{}
	for p, contents := range bundle.Files {
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
}
