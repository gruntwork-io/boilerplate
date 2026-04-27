//go:build !(js && wasm)

// Package shell provides helpers for executing external commands.
package shell

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

// RunShellCommandAndGetOutput runs the given shell command with the given environment variables and arguments in the given working directory.
func RunShellCommandAndGetOutput(l logging.Logger, workingDir string, envVars []string, argslist ...string) (string, error) {
	return RunShellCommandAndGetOutputWithContext(context.Background(), l, workingDir, envVars, argslist...)
}

// RunShellCommandAndGetOutputWithContext runs the given shell command with the given environment variables and arguments in the given working directory.
func RunShellCommandAndGetOutputWithContext(ctx context.Context, l logging.Logger, workingDir string, envVars []string, argslist ...string) (string, error) {
	command := argslist[0]
	args := argslist[1:]

	l.Debugf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Dir = workingDir

	cmd.Env = append(os.Environ(), envVars...)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// RunShellCommand runs the given shell command with the given environment variables and arguments in the given working directory.
func RunShellCommand(l logging.Logger, workingDir string, envVars []string, command string, args ...string) error {
	return RunShellCommandWithContext(context.Background(), l, workingDir, envVars, command, args...)
}

// RunShellCommandWithContext runs the given shell command with the given environment variables and arguments in the given working directory.
func RunShellCommandWithContext(ctx context.Context, l logging.Logger, workingDir string, envVars []string, command string, args ...string) error {
	l.Debugf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = workingDir

	cmd.Env = append(os.Environ(), envVars...)

	return cmd.Run()
}
