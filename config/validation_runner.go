//go:build !js || !wasm

package config

import (
	ozzo "github.com/go-ozzo/ozzo-validation"

	"github.com/gruntwork-io/boilerplate/internal/logging"
)

// DrainValidationWarnings returns and clears any warnings the runValidation
// stub recorded. In the CLI build there is no stub and validation runs for
// real, so there are never any warnings to drain — this exists only so that
// the WASM entry point (and any future non-CLI host) can share code paths
// with the CLI and still surface skipped-validation notices to callers.
func DrainValidationWarnings() []string {
	return nil
}

// runValidation applies a single custom validation rule to the given value.
// The validator is typed as any so that the signature is identical to the
// WASM stub in validation_runner_wasm.go, which cannot import ozzo.
//
// In the CLI build, validator is always populated from
// validation.CustomValidationRule.Validator (declared as ozzo.Rule), so the
// type assertion below should always succeed. The !ok branch exists only to
// flag drift — e.g. a future change to the CustomValidationRule schema that
// quietly breaks type compatibility — rather than silently passing bad input.
func runValidation(value any, validator any) error {
	rule, ok := validator.(ozzo.Rule)
	if !ok {
		logging.Logger.Printf("runValidation: validator is not an ozzo.Rule (got %T); skipping. This indicates a bug in the validation schema wiring.", validator)
		return nil
	}

	return ozzo.Validate(value, rule)
}
