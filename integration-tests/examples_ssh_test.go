//go:build ssh
// +build ssh

// This file contains tests that require SSH keys to be configured for GitHub access.
// To run these tests, use: go test -tags=ssh

package integrationtests_test

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/terratest/modules/git"
	"github.com/stretchr/testify/require"
)

func TestSSHExamplesAsRemoteTemplate(t *testing.T) {
	t.Parallel()

	branchName := git.GetCurrentBranchName(t)
	examplesBasePath := "../examples/for-learning-and-testing"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath := t.TempDir()

	examples, err := os.ReadDir(examplesBasePath)
	require.NoError(t, err)

	// Insulate the following parallel tests in a group so that cleanup routines run after all tests are done.
	t.Run("group", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" { // skip clone test for windows because of invalid file name in git
			t.Skip()
			return
		}

		for _, example := range examples {
			if !example.IsDir() {
				continue
			}

			if strings.Contains(example.Name(), "shell") {
				// This is captured in TestExamplesShell
				continue
			}

			if strings.Contains(example.Name(), "unix") {
				// unix specific case
				continue
			}

			if example.Name() == "variables" {
				t.Logf("Skipping example %s because it is implicitly tested via dependencies.", example.Name())
				continue
			}

			t.Run(path.Base(example.Name()), func(t *testing.T) {
				t.Parallel()

				templateFolder := fmt.Sprintf("git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/%s?ref=%s", example.Name(), branchName)
				outputFolder := path.Join(outputBasePath, example.Name())
				varFile := path.Join(examplesVarFilesBasePath, example.Name(), "vars.yml")
				expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, example.Name())
				testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(options.ExitWithError))
			})
		}
	})
}
