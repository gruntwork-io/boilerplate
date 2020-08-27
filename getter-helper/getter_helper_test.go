package getter_helper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/gruntwork-cli/errors"
	"github.com/gruntwork-io/terratest/modules/git"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"
)

func TestDownloadTemplatesToTempDir(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	require.NoError(t, err)
	examplePath := filepath.Join(pwd, "..", "examples", "variables")

	branch := git.GetCurrentBranchName(t)
	templateUrl := fmt.Sprintf("git@github.com:gruntwork-io/boilerplate.git//examples/variables?ref=%s", branch)
	workingDir, workPath, err := DownloadTemplatesToTemporaryFolder(templateUrl)
	defer os.RemoveAll(workingDir)
	require.NoError(t, err, errors.PrintErrorWithStackTrace(err))

	// Run diff to make sure there are no differences
	cmd := shell.Command{
		Command: "diff",
		Args:    []string{examplePath, workPath},
	}
	shell.RunCommand(t, cmd)
}
