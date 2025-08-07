package integrationtests //nolint:testpackage

import (
	"testing"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMisspelledTemplateURLErrorMessage(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/regression-test/misspelled-git"

	outputFolder := t.TempDir()

	app := cli.CreateBoilerplateCli()
	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputFolder,
		"--non-interactive",
	}
	runErr := app.Run(args)
	require.Error(t, runErr, errors.PrintErrorWithStackTrace(runErr))
	assert.Contains(t, runErr.Error(), "Did you misspell the URL")
}
