package integrationtests //nolint:testpackage

import (
	"errors"
	"testing"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/go-commons/version"
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
	require.Error(t, err)

	errUnwrapped := errors.Unwrap(err)
	var invalidBoilerplateVersion config.InvalidBoilerplateVersion
	isInvalidVersionErr := errors.As(errUnwrapped, &invalidBoilerplateVersion)
	require.True(t, isInvalidVersionErr)
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
	t.Helper()
	app := cli.CreateBoilerplateCli()

	outputPath := t.TempDir()

	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputPath,
		"--non-interactive",
	}
	return app.Run(args)
}
