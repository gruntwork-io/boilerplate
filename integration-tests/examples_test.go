package integrationtests_test

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
	"github.com/gruntwork-io/boilerplate/internal/fileutil"
	"github.com/gruntwork-io/boilerplate/internal/manifest"
	"github.com/gruntwork-io/boilerplate/options"
)

// Our integration tests run through all the examples in the /examples/for-learning-and-testing folder, generate them,
// and check that they produce the output in /test-fixtures/examples-expected-output
func TestExamples(t *testing.T) {
	t.Parallel()

	examplesBasePath := "../examples/for-learning-and-testing"
	examplesExpectedOutputBasePath := "../test-fixtures/examples-expected-output"
	examplesVarFilesBasePath := "../test-fixtures/examples-var-files"

	outputBasePath := t.TempDir()

	examples, err := os.ReadDir(examplesBasePath)
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
			t.Parallel()

			templateFolder := path.Join(examplesBasePath, example.Name())
			outputFolder := path.Join(outputBasePath, example.Name())
			varFile := path.Join(examplesVarFilesBasePath, example.Name(), "vars.yml")
			expectedOutputFolder := path.Join(examplesExpectedOutputBasePath, example.Name())

			if !fileutil.PathExists(varFile) || !fileutil.PathExists(expectedOutputFolder) {
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

func testExample(t *testing.T, templateFolder string, outputFolder string, varFile string, expectedOutputFolder string, missingKeyAction string) {
	t.Helper()

	app := cli.CreateBoilerplateCli()

	ref := "v0.12.1"
	args := []string{
		"boilerplate",
		"--template-url",
		templateFolder,
		"--output-folder",
		outputFolder,
		"--var-file",
		varFile,
		"--var",
		"RemoteBranch=" + ref,
		"--non-interactive",
		"--missing-key-action",
		missingKeyAction,
	}

	// Special handling for the shell-disabled case, which we use to test that we can disable hooks and shell helpers
	if strings.Contains(templateFolder, "shell-disabled") {
		args = append(args, "--no-hooks", "--no-shell")
	}

	err := app.Run(args)
	require.NoError(t, err)

	if expectedOutputFolder != "" {
		assertDirectoriesEqual(t, expectedOutputFolder, outputFolder)
	}
}
func TestExampleWithManifest(t *testing.T) {
	t.Parallel()

	templateFolder := "../examples/for-learning-and-testing/website"
	varFile := "../test-fixtures/examples-var-files/website/vars.yml"

	outputFolder := t.TempDir()

	app := cli.CreateBoilerplateCli()
	args := []string{
		"boilerplate",
		"--template-url", templateFolder,
		"--output-folder", outputFolder,
		"--var-file", varFile,
		"--manifest",
		"--non-interactive",
	}

	err := app.Run(args)
	require.NoError(t, err)

	// Check manifest file was created in output directory
	manifestPath := path.Join(outputFolder, "boilerplate-manifest.yaml")
	require.FileExists(t, manifestPath)

	// Read and verify manifest content
	manifestContent, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	manifestStr := string(manifestContent)
	// Verify single-generation manifest structure
	assert.Contains(t, manifestStr, "schema_version")
	assert.Contains(t, manifestStr, manifest.SchemaURL)
	assert.Contains(t, manifestStr, "timestamp")
	assert.Contains(t, manifestStr, "checksum")
	assert.Contains(t, manifestStr, "index.html")
	assert.Contains(t, manifestStr, "logo.png")
	assert.Contains(t, manifestStr, "template_url")
	assert.Contains(t, manifestStr, "boilerplate_version")
	// Verify old versioned fields are NOT present
	assert.NotContains(t, manifestStr, "latest_version")
	assert.NotContains(t, manifestStr, "versions")
}
