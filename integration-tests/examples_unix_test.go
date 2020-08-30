// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

// The following tests should only be run on unix machines

package integration_tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/terratest/modules/git"
	"github.com/stretchr/testify/require"
)

func TestExamplesShell(t *testing.T) {
	t.Parallel()

	branchName := git.GetCurrentBranchName(t)
	examplesBasePath := "../examples"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputBasePath)

	shellExamples := []string{"shell", "shell-disabled"}
	// Insulate the following parallel tests in a group so that cleanup routines run after all tests are done.
	t.Run("group", func(t *testing.T) {
		for _, example := range shellExamples {
			// Capture range variable to avoid it changing on each iteration during the tests
			example := example

			outputFolder := path.Join(outputBasePath, example)
			varFile := path.Join(examplesVarFilesBasePath, example, "vars.yml")
			expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, example)

			t.Run(example, func(t *testing.T) {
				t.Parallel()
				templateFolder := path.Join(examplesBasePath, example)
				for _, missingKeyAction := range options.AllMissingKeyActions {
					t.Run(fmt.Sprintf("%s-missing-key-%s", example, string(missingKeyAction)), func(t *testing.T) {
						testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(missingKeyAction))
					})
				}
			})

			t.Run(fmt.Sprintf("%s-remote", example), func(t *testing.T) {
				t.Parallel()
				templateFolder := fmt.Sprintf("git@github.com:gruntwork-io/boilerplate.git//examples/%s?ref=%s", example, branchName)
				testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(options.ExitWithError))
			})
		}
	})
}
