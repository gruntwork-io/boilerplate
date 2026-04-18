//go:build js && wasm

package config

import (
	"errors"

	"github.com/gruntwork-io/boilerplate/variables"
)

// getUserInput is a stub for WASM builds, which run non-interactively by
// design. It returns an error so any code path that reaches it fails loudly
// instead of hanging on a prompt that cannot be read. Callers should ensure
// options.NonInteractive is true when invoking boilerplate from WASM.
func getUserInput(_ variables.Variable) (string, error) {
	return "", errors.New("interactive prompts are not supported in the WASM build; set nonInteractive=true and provide values via vars/varFiles")
}
