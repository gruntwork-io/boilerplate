//go:build ssh && (aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris)
// +build ssh
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

// This file contains Unix-specific tests that require SSH keys to be configured for GitHub access.
// To run these tests, use: go test -tags=ssh

package integrationtests_test

import (
	"fmt"
	"path"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/terratest/modules/git"
)

func TestSSHExamplesShellRemote(t *testing.T) {
	t.Parallel()

	branchName := git.GetCurrentBranchName(t)
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath := t.TempDir()

	shellExamples := []string{"shell", "shell-disabled"}

	// Insulate the following parallel tests in a group so that cleanup routines run after all tests are done.
	t.Run("group", func(t *testing.T) {
		t.Parallel()

		for _, example := range shellExamples {
			outputFolder := path.Join(outputBasePath, example)
			varFile := path.Join(examplesVarFilesBasePath, example, "vars.yml")
			expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, example)

			t.Run(example+"-remote", func(t *testing.T) {
				t.Parallel()

				templateFolder := fmt.Sprintf("git@github.com:gruntwork-io/boilerplate.git//examples/for-learning-and-testing/%s?ref=%s", example, branchName)
				testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(options.ExitWithError))
			})
		}
	})
}
