//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
	"github.com/gruntwork-io/boilerplate/render"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", js.FuncOf(renderTemplate))

	// Block forever to keep Go runtime alive.
	select {}
}

func renderTemplate(this js.Value, args []js.Value) any {
	defer func() {
		if r := recover(); r != nil {
			// Panic recovery is best-effort; the JS caller will see undefined.
			fmt.Println("boilerplate: recovered from panic:", r)
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

	result, err := render.RenderTemplateFromString(logging.Discard(), vfs.NewMemMapFS(), "template", templateStr, variables, opts)
	if err != nil {
		return js.Global().Get("Error").New(fmt.Sprintf("template rendering failed: %v", err))
	}

	return result
}
