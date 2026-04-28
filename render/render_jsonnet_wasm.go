//go:build js && wasm

package render

import (
	"errors"

	"github.com/gruntwork-io/boilerplate/options"
)

// RenderJsonnetTemplate is a WASM stub: go-jsonnet does not compile under
// GOOS=js.
func RenderJsonnetTemplate(_ string, _ map[string]any, _ *options.BoilerplateOptions) (string, error) {
	return "", errors.New("jsonnet template rendering is not supported in the WASM build")
}
