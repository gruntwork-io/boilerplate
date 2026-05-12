//go:build js && wasm

// The full WASM build registers boilerplateRenderTemplate,
// boilerplateInputsMap, boilerplateRenderFile, and boilerplateRenderFiles.
// It pulls in the config package and its dependencies, so the binary is
// substantially larger than the lite build.
package main

import (
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/inputs"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/render"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/renderfile"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/renderfiles"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", render.Handler())
	js.Global().Set("boilerplateInputsMap", inputs.Handler())
	js.Global().Set("boilerplateRenderFile", renderfile.Handler())
	js.Global().Set("boilerplateRenderFiles", renderfiles.Handler())

	// Block forever to keep Go runtime alive.
	select {}
}
