//go:build windows
// +build windows

package integration_tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/stretchr/testify/require"
)

func TestWindowsExamples(t *testing.T) {
	t.Parallel()

	examplesBasePath := "../examples/for-learning-and-testing"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputBasePath)

	windowsExamples := []string{"dependencies"}
	t.Run("group", func(t *testing.T) {
		for _, example := range windowsExamples {
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
		}
	})
}
