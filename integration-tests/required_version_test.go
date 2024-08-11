package integration_tests

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/go-commons/errors"
	"github.com/gruntwork-io/go-commons/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testVersion = "v1.33.7"
)

func TestRequiredVersionMatchCase(t *testing.T) {
	t.Parallel()

	// Make sure that the test is run with the ld flags setting version to our expected test version.
	require.Equal(t, testVersion, version.GetVersion())

	require.NoError(
		t,
		runRequiredVersionExample(t, "../test-fixtures/regression-test/required-version/match"),
	)
}

func TestRequiredVersionOverTest(t *testing.T) {
	t.Parallel()

	// Make sure that the test is run with the ld flags setting version to our expected test version.
	require.Equal(t, testVersion, version.GetVersion())

	err := runRequiredVersionExample(t, "../test-fixtures/regression-test/required-version/over-test")
	assert.Error(t, err)

	errUnwrapped := errors.Unwrap(err)
	_, isInvalidVersionErr := errUnwrapped.(config.InvalidBoilerplateVersion)
	assert.True(t, isInvalidVersionErr)
}

func TestRequiredVersionUnderTest(t *testing.T) {
	t.Parallel()

	// Make sure that the test is run with the ld flags setting version to our expected test version.
	require.Equal(t, testVersion, version.GetVersion())

	require.NoError(
		t,
		runRequiredVersionExample(t, "../test-fixtures/regression-test/required-version/under-test"),
	)
}

func runRequiredVersionExample(t *testing.T, templateFolder string) error {
	app := cli.CreateBoilerplateCli()

	outputPath, err := ioutil.TempDir("", "boilerplate-test-output-reqver")
	require.NoError(t, err)
	defer os.RemoveAll(outputPath)

	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputPath,
		"--non-interactive",
		"--silent",
	}
	return app.Run(args)
}
