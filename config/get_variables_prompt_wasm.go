//go:build js && wasm

package config

import (
	"fmt"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/variables"
)

// Interactive prompt functions are unavailable in WASM. Code paths reaching
// these stubs always pair with NonInteractive=true (the only mode supported in
// the WASM build), so they should not be hit at runtime; they exist only to
// satisfy the linker.

func getVariableFromUser(_ logging.Logger, variable variables.Variable, _ variables.InvalidEntries) (any, error) {
	return nil, fmt.Errorf("interactive prompts are not supported in WASM (variable %q)", variable.FullName())
}
