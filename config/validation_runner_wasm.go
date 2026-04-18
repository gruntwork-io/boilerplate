//go:build js && wasm

package config

// runValidation is a stub for WASM builds. Validation rules carry
// ozzo-validation.Rule objects, but the ozzo package pulls in crypto and
// reflection code that is excluded from the WASM binary (see
// validation/validation_wasm.go). Custom validations are never executed at
// runtime in the WASM environment.
func runValidation(_ any, _ any) error {
	return nil
}
