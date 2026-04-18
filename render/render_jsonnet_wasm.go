//go:build js && wasm

package render

import (
	"errors"

	"github.com/gruntwork-io/boilerplate/options"
)

// RenderJsonnetTemplate is a stub for WASM builds. The google/go-jsonnet
// library pulls in dependencies that do not compile under GOOS=js, and
// downstream consumers of the WASM entry point only render Go templates. If
// a template folder contains a .jsonnet file when run from WASM, this returns
// a clear error rather than silently skipping the file.
func RenderJsonnetTemplate(_ string, _ map[string]any, _ *options.BoilerplateOptions) (string, error) {
	return "", errors.New("jsonnet template rendering is not supported in the WASM build")
}
