// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package integration_tests

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/shell"
)

// Diffing two directories to ensure they have the exact same files, contents, etc and showing exactly what's different
// takes a lot of code. Why waste time on that when this functionality is already nicely implemented in the Unix/Linux
// "diff" command? We shell out to that command at test time.
func assertDirectoriesEqual(t *testing.T, folderWithExpectedContents string, folderWithActualContents string) {
	cmd := shell.Command{
		Command: "diff",
		Args:    []string{"-r", "-u", folderWithExpectedContents, folderWithActualContents},
	}
	shell.RunCommand(t, cmd)
}
