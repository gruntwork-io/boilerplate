package integration_tests

import (
	"testing"
	"io/ioutil"
	"github.com/stretchr/testify/assert"
	"github.com/gruntwork-io/boilerplate/cli"
	"fmt"
	"path"
	"os/exec"
	"os"
	"strings"
	"github.com/gruntwork-io/boilerplate/config"
)

// Our integration tests run through all the examples in the /examples folder, generate them, and check that they
// produce the output in /test-fixtures/examples-expected-output
func TestExamples(t *testing.T) {
	t.Parallel()

	examplesBasePath := "../examples"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath, err := ioutil.TempDir("", "boilerplate-test-output")
	assert.Nil(t, err)
	defer os.Remove(outputBasePath)

	files, err := ioutil.ReadDir(examplesBasePath)
	assert.Nil(t, err)

	app := cli.CreateBoilerplateCli("test")

	for _, file := range files {
		for _, missingKeyAction := range config.ALL_MISSING_KEY_ACTIONS {
			if file.IsDir() {
				templateFolder := path.Join(examplesBasePath, file.Name())
				outputFolder := path.Join(outputBasePath, file.Name())
				varFile := path.Join(examplesVarFilesBasePath, file.Name(), "vars.yml")
				expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, file.Name())

				command := fmt.Sprintf("boilerplate --template-folder %s --output-folder %s --var-file %s --non-interactive --missing-key-action %s", templateFolder, outputFolder, varFile, missingKeyAction.String())
				err := app.Run(strings.Split(command, " "))
				assert.Nil(t, err, "boilerplate exited with an error when trying to generate example %s: %s", templateFolder, err)
				assertDirectoriesEqual(t, expectedOutputFolder, outputFolder)
			}
		}
	}
}

// Diffing two directories to ensure they have the exact same files, contents, etc and showing exactly what's different
// takes a lot of code. Why waste time on that when this functionality is already nicely implemented in the Unix/Linux
// "diff" command? We shell out to that command at test time.
func assertDirectoriesEqual(t *testing.T, folderWithExpectedContents string, folderWithActualContents string) {
	cmd := exec.Command("diff", "-u", folderWithExpectedContents, folderWithActualContents)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	assert.Nil(t, err, "diff command exited with an error. This likely means the contents of %s and %s are different. The log output above should show the diff.", folderWithExpectedContents, folderWithActualContents)
}