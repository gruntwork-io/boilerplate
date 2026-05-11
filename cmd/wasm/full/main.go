//go:build js && wasm

// The full WASM build registers both boilerplateRenderTemplate and
// boilerplateInputsMap. It pulls in the config package and its dependencies,
// so the binary is substantially larger than the lite build.
package main

import (
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/inputs"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/render"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", render.Handler())
	js.Global().Set("boilerplateInputsMap", inputs.Handler())

	// Block forever to keep Go runtime alive.
	select {}
}
