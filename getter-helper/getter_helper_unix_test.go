//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package getter_helper

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/terratest/modules/git"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"
)

func TestDownloadTemplatesToTempDir(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	require.NoError(t, err)
	examplePath := filepath.Join(pwd, "..", "examples", "for-learning-and-testing", "variables")

	branch := git.GetCurrentBranchName(t)
	templateUrl := fmt.Sprintf("git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/variables?ref=%s", branch)
	workingDir, workPath, err := DownloadTemplatesToTemporaryFolder(templateUrl, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	defer os.RemoveAll(workingDir)
	require.NoError(t, err, errors.PrintErrorWithStackTrace(err))

	// Run diff to make sure there are no differences
	cmd := shell.Command{
		Command: "diff",
		Args:    []string{examplePath, workPath},
	}
	shell.RunCommand(t, cmd)
}
