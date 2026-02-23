//go:build js && wasm

package shell

import (
	"context"
	"errors"
)

var errShellNotSupported = errors.New("shell commands are not supported in WASM")

// RunShellCommandAndGetOutput is a stub that returns an error in WASM builds.
func RunShellCommandAndGetOutput(workingDir string, envVars []string, argslist ...string) (string, error) {
	return "", errShellNotSupported
}

// RunShellCommandAndGetOutputWithContext is a stub that returns an error in WASM builds.
func RunShellCommandAndGetOutputWithContext(ctx context.Context, workingDir string, envVars []string, argslist ...string) (string, error) {
	return "", errShellNotSupported
}

// RunShellCommand is a stub that returns an error in WASM builds.
func RunShellCommand(workingDir string, envVars []string, command string, args ...string) error {
	return errShellNotSupported
}

// RunShellCommandWithContext is a stub that returns an error in WASM builds.
func RunShellCommandWithContext(ctx context.Context, workingDir string, envVars []string, command string, args ...string) error {
	return errShellNotSupported
}
