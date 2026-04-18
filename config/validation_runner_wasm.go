//go:build js && wasm

package config

import "sync"

// runValidation is a stub for WASM builds. Validation rules carry
// ozzo-validation.Rule objects, but the ozzo package pulls in crypto and
// reflection code that is excluded from the WASM binary (see
// validation/validation_wasm.go). Custom validations are never executed at
// runtime in the WASM environment.
//
// Rather than silently no-op, record a single deduplicated warning so the
// WASM entry point can surface it to callers via the Promise result.
func runValidation(_ any, _ any) error {
	validationWarningOnce.Do(func() {
		validationWarningMu.Lock()
		defer validationWarningMu.Unlock()

		validationWarnings = append(validationWarnings, "custom variable validations are not enforced in the WASM build")
	})

	return nil
}

var (
	validationWarningMu   sync.Mutex
	validationWarningOnce sync.Once
	validationWarnings    []string
)

// DrainValidationWarnings returns and clears any warnings recorded by
// runValidation. Callers are expected to invoke it exactly once per
// ProcessTemplate run and include the result in the JS response. The
// sync.Once is reset so subsequent runs start fresh.
func DrainValidationWarnings() []string {
	validationWarningMu.Lock()
	defer validationWarningMu.Unlock()

	out := validationWarnings
	validationWarnings = nil
	validationWarningOnce = sync.Once{}

	return out
}
