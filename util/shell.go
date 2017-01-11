package util

import (
	"os/exec"
	"os"
	"strings"
	"github.com/gruntwork-io/boilerplate/errors"
)

// Run the given shell command with the given arguments in the given working directory
func RunShellCommandAndGetOutput(workingDir string, command string, args ... string) (string, error) {
	Logger.Printf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Dir = workingDir

	out, err := cmd.Output()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return string(out), nil
}
