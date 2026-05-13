//go:build !(js && wasm)

package config

import ozzo "github.com/go-ozzo/ozzo-validation"

// runValidator dispatches to ozzo-validation in non-WASM builds.
//
// validator's static type is `any` because validation.CustomValidationRule has
// different shapes for CLI vs WASM (see validation/validation_wasm.go); we
// type-assert to ozzo.Rule here so a single call site in get_variables.go can
// compile under both build tags.
func runValidator(value any, validator any) error {
	rule, ok := validator.(ozzo.Rule)
	if !ok {
		return nil
	}

	return ozzo.Validate(value, rule)
}
