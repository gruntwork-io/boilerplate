//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

// Package integrationtests provides integration tests for the boilerplate tool.
package integrationtests //nolint:testpackage

import (
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/files"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"
)

// Diffing two directories to ensure they have the exact same files, contents, etc and showing exactly what's different
// takes a lot of code. Why waste time on that when this functionality is already nicely implemented in the Unix/Linux
// "diff" command? We shell out to that command at test time.
func assertDirectoriesEqual(t *testing.T, folderWithExpectedContents string, folderWithActualContents string) {
	t.Helper()
	// Copy the folder contents to a temp dir for testing purposes without .keep-dir files, which are used to ensure the
	// directory exists in git.
	tmpFolder, err := files.CopyFolderToTemp(folderWithExpectedContents, "boilerplate-assert-direq-", func(path string) bool {
		return !strings.HasSuffix(path, ".keep-dir")
	})
	require.NoError(t, err)

	cmd := shell.Command{
		Command: "diff",
		Args:    []string{"-r", "-u", tmpFolder, folderWithActualContents},
	}
	shell.RunCommand(t, cmd)
}
