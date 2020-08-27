package integration_tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/gruntwork-io/terratest/modules/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/util"
)

// Our integration tests run through all the examples in the /examples folder, generate them, and check that they
// produce the output in /test-fixtures/examples-expected-output
func TestExamples(t *testing.T) {
	t.Parallel()

	examplesBasePath := "../examples"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.Remove(outputBasePath)

	examples, err := ioutil.ReadDir(examplesBasePath)
	require.NoError(t, err)

	for _, example := range examples {
		if !example.IsDir() {
			continue
		}

		t.Run(path.Base(example.Name()), func(t *testing.T) {
			templateFolder := path.Join(examplesBasePath, example.Name())
			outputFolder := path.Join(outputBasePath, example.Name())
			varFile := path.Join(examplesVarFilesBasePath, example.Name(), "vars.yml")
			expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, example.Name())

			if !util.PathExists(varFile) || !util.PathExists(expectedOutputFolder) {
				t.Logf("Skipping example %s because either the var file (%s) or expected output folder (%s) does not exist.", templateFolder, varFile, expectedOutputFolder)
				return
			}

			for _, missingKeyAction := range options.AllMissingKeyActions {
				t.Run(fmt.Sprintf("%s-missing-key-%s", example.Name(), string(missingKeyAction)), func(t *testing.T) {
					testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(missingKeyAction))
				})
			}
		})
	}
}

func TestExamplesAsRemoteTemplate(t *testing.T) {
	t.Parallel()

	branchName := git.GetCurrentBranchName(t)
	examplesBasePath := "../examples"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.Remove(outputBasePath)

	examples, err := ioutil.ReadDir(examplesBasePath)
	require.NoError(t, err)

	for _, example := range examples {
		if !example.IsDir() {
			continue
		}

		t.Run(path.Base(example.Name()), func(t *testing.T) {
			templateFolder := fmt.Sprintf("git@github.com:gruntwork-io/boilerplate.git//examples/%s?ref=%s", example.Name(), branchName)
			outputFolder := path.Join(outputBasePath, example.Name())
			varFile := path.Join(examplesVarFilesBasePath, example.Name(), "vars.yml")
			expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, example.Name())
			testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(options.ExitWithError))
		})
	}

}

func testExample(t *testing.T, templateFolder string, outputFolder string, varFile string, expectedOutputFolder string, missingKeyAction string) {
	app := cli.CreateBoilerplateCli("test")

	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputFolder,
		"--var-file",
		varFile,
		"--non-interactive",
		"--missing-key-action",
		missingKeyAction,
	}

	// Special handling for the shell-disabled case, which we use to test that we can disable hooks and shell helpers
	if strings.Contains(templateFolder, "shell-disabled") {
		args = append(args, "--disable-hooks", "--disable-shell")
	}

	err := app.Run(args)
	assert.NoError(t, err, errors.PrintErrorWithStackTrace(err))
	assertDirectoriesEqual(t, expectedOutputFolder, outputFolder)
}

// Diffing two directories to ensure they have the exact same files, contents, etc and showing exactly what's different
// takes a lot of code. Why waste time on that when this functionality is already nicely implemented in the Unix/Linux
// "diff" command? We shell out to that command at test time.
func assertDirectoriesEqual(t *testing.T, folderWithExpectedContents string, folderWithActualContents string) {
	cmd := exec.Command("diff", "-r", "-u", folderWithExpectedContents, folderWithActualContents)

	_, err := cmd.Output()
	assert.NoError(t, err)
}
