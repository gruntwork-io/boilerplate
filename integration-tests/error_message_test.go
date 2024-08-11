package integration_tests

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMisspelledTemplateURLErrorMessage(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/regression-test/misspelled-git"

	outputFolder, err := ioutil.TempDir("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputFolder)

	app := cli.CreateBoilerplateCli()
	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputFolder,
		"--non-interactive",
		"--silent",
	}
	runErr := app.Run(args)
	assert.Error(t, runErr, errors.PrintErrorWithStackTrace(runErr))
	assert.Contains(t, runErr.Error(), "Did you misspell the URL")
}
