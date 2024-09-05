package integration_tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/errors"
)

// Our integration tests run through all the examples in the /examples/for-learning-and-testing folder, generate them,
// and check that they produce the output in /test-fixtures/examples-expected-output
func TestEnvVarExample(t *testing.T) {
	app := cli.CreateBoilerplateCli()

	tempdir := t.TempDir()

	args := []string{
		"boilerplate",
		"--template-url",
		"../test-fixtures/env-vars",
		"--output-folder",
		tempdir,
		"--non-interactive",
	}

	err := app.Run(args)
	require.NoError(t, err, errors.PrintErrorWithStackTrace(err))

	testTxt := tempdir + "/target.txt"
	assert.FileExists(t, testTxt)
	content, err := os.ReadFile(testTxt)
	require.NoError(t, err)
	assert.Equal(t, "default-value\n", string(content))

	t.Setenv("BOILERPLATE_ValueFromEnvVar", "env-var-value")
	err = app.Run(args)
	require.NoError(t, err, errors.PrintErrorWithStackTrace(err))

	content, err = os.ReadFile(testTxt)
	require.NoError(t, err)
	assert.Equal(t, "env-var-value\n", string(content))
}
