//go:build js && wasm

package config

import (
	"sync"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

// runValidation is a stub: ozzo is excluded from the WASM binary (see
// validation/validation_wasm.go). Records a deduplicated warning that the
// entry point drains into the Promise result.
func runValidation(_ logging.Logger, _ any, _ any) error {
	validationWarningMu.Lock()
	defer validationWarningMu.Unlock()

	if !validationWarned {
		validationWarnings = append(validationWarnings, "custom variable validations are not enforced in the WASM build")
		validationWarned = true
	}

	return nil
}

var (
	validationWarningMu sync.Mutex
	validationWarned    bool
	validationWarnings  []string
)

// DrainValidationWarnings returns and clears any warnings recorded by
// runValidation, resetting the dedupe flag for the next run.
func DrainValidationWarnings() []string {
	validationWarningMu.Lock()
	defer validationWarningMu.Unlock()

	out := validationWarnings
	validationWarnings = nil
	validationWarned = false

	return out
}
