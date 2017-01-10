package util

import (
	"os/exec"
	"os"
	"strings"
	"github.com/gruntwork-io/boilerplate/errors"
)

// Run the given shell command with the given arguments
func RunShellCommandAndGetOutput(command string, args ... string) (string, error) {
	Logger.Printf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)

	cmd.Stdin = os.Stdin

	out, err := cmd.Output()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	return string(out), nil
}
