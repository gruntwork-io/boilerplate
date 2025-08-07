//go:build windows
// +build windows

package integration_tests

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"
)

// Like Unix/Linux, diffing two files is relatively easier than golang native functions in powershell.
// Get the contents of each folder and then compare them.
// Inspired by https://devblogs.microsoft.com/scripting/easily-compare-two-folders-by-using-powershell/
func assertDirectoriesEqual(t *testing.T, folderWithExpectedContents string, folderWithActualContents string) {
	powershellDiffTemplate := `$fone = Get-ChildItem -Recurse -path %s
$ftwo = Get-ChildItem -Recurse -path %s
Compare-Object -ReferenceObject $fone -DifferenceObject $ftwo
`
	powershellDiffCmd := fmt.Sprintf(powershellDiffTemplate, folderWithExpectedContents, folderWithActualContents)
	runPowershell(t, powershellDiffCmd)
}

func runPowershell(t *testing.T, args ...string) {
	ps, err := exec.LookPath("powershell.exe")
	require.NoError(t, err)

	psArgs := append([]string{"-NoProfile", "-NonInteractive"}, args...)
	cmd := shell.Command{
		Command: ps,
		Args:    psArgs,
	}
	shell.RunCommand(t, cmd)
}
