package integration_tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/errors"
)

// Test JSON variable file support specifically
func TestJsonVarFileSupport(t *testing.T) {
	t.Parallel()

	// Create temporary directories for template and output
	templateFolder, err := os.MkdirTemp("", "boilerplate-json-template")
	require.NoError(t, err)
	defer os.RemoveAll(templateFolder)

	outputFolder, err := os.MkdirTemp("", "boilerplate-json-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputFolder)

	// Create a simple boilerplate.yml
	boilerplateConfig := `variables:
  - name: Name
    type: string
  - name: Active
    type: bool`

	err = os.WriteFile(filepath.Join(templateFolder, "boilerplate.yml"), []byte(boilerplateConfig), 0644)
	require.NoError(t, err)

	// Create a simple template file
	templateContent := `Hello {{ .Name }}!
Your active status is: {{ .Active }}.`
	
	err = os.WriteFile(filepath.Join(templateFolder, "greeting.txt"), []byte(templateContent), 0644)
	require.NoError(t, err)

	// Create JSON variable file
	jsonVars := `{
  "Name": "John Doe",
  "Active": true
}`

	varFile := filepath.Join(templateFolder, "vars.json")
	err = os.WriteFile(varFile, []byte(jsonVars), 0644)
	require.NoError(t, err)

	// Run boilerplate with JSON variable file
	app := cli.CreateBoilerplateCli()
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
		"error",
	}

	err = app.Run(args)
	require.NoError(t, err, errors.PrintErrorWithStackTrace(err))

	// Verify the output
	outputFile := filepath.Join(outputFolder, "greeting.txt")
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	expectedContent := `Hello John Doe!
Your active status is: true.`

	assert.Equal(t, expectedContent, string(content))
}