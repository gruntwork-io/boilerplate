package util

import (
	"os"
	"os/exec"
	"strings"

	"github.com/gruntwork-io/boilerplate/errors"
)

// Run the given shell command with the given environment variables and arguments in the given working directory
func RunShellCommandAndGetOutput(workingDir string, envVars []string, argslist ...string) (string, error) {
	command := argslist[0]
	args := argslist[1:]
	Logger.Printf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), envVars...)

	out, err := cmd.Output()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return string(out), nil
}

// Run the given shell command with the given environment variables and arguments in the given working directory
func RunShellCommand(workingDir string, envVars []string, command string, args ...string) error {
	Logger.Printf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), envVars...)

	return cmd.Run()
}
