//go:build !js || !wasm

package config

import (
	ozzo "github.com/go-ozzo/ozzo-validation"
)

// runValidation applies a single custom validation rule to the given value.
// The validator is typed as any so that the signature is identical to the
// WASM stub in validation_runner_wasm.go, which cannot import ozzo.
func runValidation(value any, validator any) error {
	rule, ok := validator.(ozzo.Rule)
	if !ok {
		return nil
	}

	return ozzo.Validate(value, rule)
}
