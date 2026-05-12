//go:build js && wasm

// The full WASM build registers boilerplateRenderTemplate,
// boilerplateInputsMap, boilerplateRenderFile, boilerplateRenderFiles,
// and the prepared-bundle trio (boilerplatePrepareBundle,
// boilerplateRenderFilesWithHandle, boilerplateReleaseBundle). It
// pulls in the config package and its dependencies, so the binary is
// substantially larger than the lite build.
package main

import (
	"syscall/js"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/inputs"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/preparedbundle"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/render"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/renderfile"
	"github.com/gruntwork-io/boilerplate/cmd/wasm/renderfiles"
)

func main() {
	js.Global().Set("boilerplateRenderTemplate", render.Handler())
	js.Global().Set("boilerplateInputsMap", inputs.Handler())
	js.Global().Set("boilerplateRenderFile", renderfile.Handler())
	js.Global().Set("boilerplateRenderFiles", renderfiles.Handler())

	// The prepared-bundle handlers share a single handle store so a
	// handle returned by Prepare resolves correctly inside the matching
	// RenderFilesWithHandle / Release call. Wired up exactly once at
	// main() time.
	pb := preparedbundle.New()

	js.Global().Set("boilerplatePrepareBundle", pb.Prepare)
	js.Global().Set("boilerplateRenderFilesWithHandle", pb.RenderFilesWithHandle)
	js.Global().Set("boilerplateReleaseBundle", pb.Release)

	// Block forever to keep Go runtime alive.
	select {}
}
