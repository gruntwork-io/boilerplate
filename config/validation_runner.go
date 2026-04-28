//go:build !(js && wasm)

package config

import (
	ozzo "github.com/go-ozzo/ozzo-validation"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

// DrainValidationWarnings is a no-op in the CLI build; it exists so the WASM
// build can share the same call sites.
func DrainValidationWarnings() []string {
	return nil
}

// runValidation uses any-typed parameters to match the WASM stub's signature,
// which cannot import ozzo.
func runValidation(l logging.Logger, value any, validator any) error {
	rule, ok := validator.(ozzo.Rule)
	if !ok {
		l.Warnf("runValidation: validator is not an ozzo.Rule (got %T); skipping", validator)
		return nil
	}

	return ozzo.Validate(value, rule)
}
