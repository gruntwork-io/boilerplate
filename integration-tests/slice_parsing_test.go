package integration_tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that we can pass --var values with commas and spaces, and that those are parsed as a single value, rather than
// multiple.
func TestSliceParsing(t *testing.T) {
	t.Parallel()

	templateFolder := "../test-fixtures/regression-test/slice-parsing"

	outputFolder, err := os.MkdirTemp("", "boilerplate-test-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputFolder)

	mapValue := `{"key1":"value1","key2":"value2","key3":"value3"}`

	app := cli.CreateBoilerplateCli()
	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputFolder,
		"--var",
		fmt.Sprintf("MapValue=%s", mapValue),
		"--non-interactive",
		"--silent",
	}

	runErr := app.Run(args)
	require.NoError(t, runErr)

	outputPath := filepath.Join(outputFolder, "output.txt")

	// Check the JSON we passed in via the CLI got through without any modifications
	bytes, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, mapValue, string(bytes))
}
