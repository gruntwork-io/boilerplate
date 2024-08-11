package integration_tests

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
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

// Our integration tests run through all the examples in the /examples/for-learning-and-testing folder, generate them,
// and check that they produce the output in /test-fixtures/examples-expected-output
func TestExamples(t *testing.T) {
	t.Parallel()

	examplesBasePath := "../examples/for-learning-and-testing"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputBasePath)

	examples, err := ioutil.ReadDir(examplesBasePath)
	require.NoError(t, err)

	for _, example := range examples {
		if !example.IsDir() {
			continue
		}
		if strings.Contains(example.Name(), "shell") {
			// This is captured in TestExamplesShell
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

			if runtime.GOOS != "windows" { // skip clone test for windows because of invalid file name in git
				for _, missingKeyAction := range options.AllMissingKeyActions {
					t.Run(fmt.Sprintf("%s-missing-key-%s", example.Name(), string(missingKeyAction)), func(t *testing.T) {
						testExample(t, templateFolder, outputFolder, varFile, expectedOutputFolder, string(missingKeyAction))
					})
				}
			}
		})
	}
}

func TestExamplesAsRemoteTemplate(t *testing.T) {
	t.Parallel()

	branchName := git.GetCurrentBranchName(t)
	examplesBasePath := "../examples/for-learning-and-testing"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputBasePath)

	examples, err := ioutil.ReadDir(examplesBasePath)
	require.NoError(t, err)

	// Insulate the following parallel tests in a group so that cleanup routines run after all tests are done.
	t.Run("group", func(t *testing.T) {
		if runtime.GOOS == "windows" { // skip clone test for windows because of invalid file name in git
			t.Skip()
			return
		}
		for _, example := range examples {
			// Capture range variable to avoid it changing on each iteration during the tests
			example := example

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

func testExample(t *testing.T, templateFolder string, outputFolder string, varFile string, expectedOutputFolder string, missingKeyAction string) {
	app := cli.CreateBoilerplateCli()

	ref := git.GetCurrentGitRef(t)
	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputFolder,
		"--var-file",
		varFile,
		"--var",
		fmt.Sprintf("RemoteBranch=%s", ref),
		"--non-interactive",
		"--missing-key-action",
		missingKeyAction,
		"--silent",
	}

	// Special handling for the shell-disabled case, which we use to test that we can disable hooks and shell helpers
	if strings.Contains(templateFolder, "shell-disabled") {
		args = append(args, "--disable-hooks", "--disable-shell")
	}

	err := app.Run(args)
	assert.NoError(t, err, errors.PrintErrorWithStackTrace(err))
	if expectedOutputFolder != "" {
		assertDirectoriesEqual(t, expectedOutputFolder, outputFolder)
	}
}
