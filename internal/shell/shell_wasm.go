//go:build js && wasm

package shell

import (
	"context"
	"errors"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

var errShellNotSupported = errors.New("shell commands are not supported in WASM")

// RunShellCommandAndGetOutput is a stub that returns an error in WASM builds.
func RunShellCommandAndGetOutput(_ logging.Logger, _ string, _ []string, _ ...string) (string, error) {
	return "", errShellNotSupported
}

// RunShellCommandAndGetOutputWithContext is a stub that returns an error in WASM builds.
func RunShellCommandAndGetOutputWithContext(_ context.Context, _ logging.Logger, _ string, _ []string, _ ...string) (string, error) {
	return "", errShellNotSupported
}

// RunShellCommand is a stub that returns an error in WASM builds.
func RunShellCommand(_ logging.Logger, _ string, _ []string, _ string, _ ...string) error {
	return errShellNotSupported
}

// RunShellCommandWithContext is a stub that returns an error in WASM builds.
func RunShellCommandWithContext(_ context.Context, _ logging.Logger, _ string, _ []string, _ string, _ ...string) error {
	return errShellNotSupported
}
