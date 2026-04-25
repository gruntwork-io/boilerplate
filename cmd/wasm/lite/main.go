//go:build js && wasm

// The lite WASM build registers only boilerplateRenderTemplate. It avoids
// importing the templates package and its transitive dependencies, keeping
// the binary small for callers that only need string rendering.
package main

import (
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/render"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", render.Handler())

	// Block forever to keep Go runtime alive.
	select {}
}
