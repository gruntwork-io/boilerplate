//go:build js && wasm

package config

import (
	"errors"

	"github.com/gruntwork-io/boilerplate/variables"
)

// getUserInput errors out loudly rather than hanging on a prompt that cannot
// be read. Callers must set options.NonInteractive in WASM.
func getUserInput(_ variables.Variable) (string, error) {
	return "", errors.New("interactive prompts are not supported in the WASM build; set nonInteractive=true and provide values via vars/varFiles")
}
