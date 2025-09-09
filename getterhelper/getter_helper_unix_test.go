//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package getterhelper_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terratest/modules/git"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/getterhelper"
)

func TestDownloadTemplatesToTempDir(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	require.NoError(t, err)

	examplePath := filepath.Join(pwd, "..", "examples", "for-learning-and-testing", "variables")

	branch := git.GetCurrentBranchName(t)
	templateURL := "git::https://github.com/gruntwork-io/boilerplate.git//examples/for-learning-and-testing/variables?ref=" + branch

	workingDir, workPath, err := getterhelper.DownloadTemplatesToTemporaryFolder(templateURL)
	defer os.RemoveAll(workingDir)

	require.NoError(t, err, errors.PrintErrorWithStackTrace(err))

	// Run diff to make sure there are no differences
	cmd := shell.Command{
		Command: "diff",
		Args:    []string{examplePath, workPath},
	}
	shell.RunCommand(t, cmd)
}
