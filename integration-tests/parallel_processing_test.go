package integration_tests

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
)

// Test that parallel processing produces the same output as sequential processing
func TestParallelProcessingProducesSameOutput(t *testing.T) {
	t.Parallel()

	templateFolder := "../examples/for-learning-and-testing/dependencies-for-each"
	varFile := "../test-fixtures/examples-var-files/dependencies-for-each/vars.yml"

	sequentialOutput, err := os.MkdirTemp("", "boilerplate-test-sequential")
	require.NoError(t, err)
	defer os.RemoveAll(sequentialOutput)

	parallelOutput, err := os.MkdirTemp("", "boilerplate-test-parallel")
	require.NoError(t, err)
	defer os.RemoveAll(parallelOutput)

	// Run both sequential and parallel processing
	runBoilerplateWithParallelFlag(t, templateFolder, sequentialOutput, varFile, false)
	runBoilerplateWithParallelFlag(t, templateFolder, parallelOutput, varFile, true)

	// Compare outputs - they should be identical
	assertDirectoriesEqual(t, sequentialOutput, parallelOutput)
}



// Test that parallel processing handles errors correctly from multiple dependencies
func TestParallelProcessingErrorHandling(t *testing.T) {
	t.Parallel()

	templateDir := createFailingTemplate(t)
	defer os.RemoveAll(templateDir)

	outputDir, err := os.MkdirTemp("", "boilerplate-error-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputDir)

	// Run with parallel processing - should fail but handle errors gracefully
	app := cli.CreateBoilerplateCli()
	args := []string{
		"boilerplate",
		"--template-url", templateDir,
		"--output-folder", outputDir,
		"--non-interactive",
		"--parallel-for-each",
	}

	err = app.Run(args)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-template")
}



// Test that the --parallel-for-each flag is properly propagated to nested dependencies
func TestParallelFlagPropagation(t *testing.T) {
	t.Parallel()

	templateDir := createNestedTemplate(t)
	defer os.RemoveAll(templateDir)

	outputDir, err := os.MkdirTemp("", "boilerplate-propagation-output")
	require.NoError(t, err)
	defer os.RemoveAll(outputDir)

	// Run with parallel processing
	runBoilerplateWithParallelFlag(t, templateDir, outputDir, "", true)

	// Verify that all nested files were created
	expectedPaths := []string{
		"parent/parent1/child/child1/result.txt",
		"parent/parent1/child/child2/result.txt", 
		"parent/parent1/child/child3/result.txt",
		"parent/parent2/child/child1/result.txt",
		"parent/parent2/child/child2/result.txt",
		"parent/parent2/child/child3/result.txt",
	}

	for _, expectedPath := range expectedPaths {
		fullPath := path.Join(outputDir, expectedPath)
		assert.FileExists(t, fullPath, "Nested file should exist at %s", expectedPath)
	}
}



// Helper function to run boilerplate with or without parallel flag
func runBoilerplateWithParallelFlag(t *testing.T, templateFolder, outputFolder, varFile string, parallel bool) {
	app := cli.CreateBoilerplateCli()
	
	args := []string{
		"boilerplate",
		"--template-url", templateFolder,
		"--output-folder", outputFolder,
		"--non-interactive",
	}

	if varFile != "" {
		args = append(args, "--var-file", varFile)
	}

	if parallel {
		args = append(args, "--parallel-for-each")
	}

	err := app.Run(args)
	require.NoError(t, err, "Boilerplate execution should succeed")
}



// Helper function to create a template that will cause failures
func createFailingTemplate(t *testing.T) string {
	templateDir, err := os.MkdirTemp("", "boilerplate-error-test")
	require.NoError(t, err)

	boilerplateConfig := `dependencies:
  - name: failing-dependency-1
    template-url: ./nonexistent-template-1
    for_each:
      - item1
      - item2
    output-folder: "output/{{ .__each__ }}"
  - name: failing-dependency-2
    template-url: ./nonexistent-template-2
    for_each:
      - item3
      - item4
    output-folder: "output/{{ .__each__ }}"
`

	err = os.WriteFile(path.Join(templateDir, "boilerplate.yml"), []byte(boilerplateConfig), 0o644)
	require.NoError(t, err)

	return templateDir
}



// Helper function to create a template with nested dependencies
func createNestedTemplate(t *testing.T) string {
	templateDir, err := os.MkdirTemp("", "boilerplate-propagation-test")
	require.NoError(t, err)

	// Create main template with for_each dependency
	err = os.WriteFile(path.Join(templateDir, "boilerplate.yml"), []byte(`dependencies:
  - name: parent-dependency
    template-url: ./nested-template
    for_each:
      - parent1
      - parent2
    output-folder: "parent/{{ .__each__ }}"
`), 0o644)
	require.NoError(t, err)

	// Create nested template directory
	nestedTemplateDir := path.Join(templateDir, "nested-template")
	err = os.MkdirAll(nestedTemplateDir, 0o755)
	require.NoError(t, err)

	// Create nested template with its own for_each dependencies
	err = os.WriteFile(path.Join(nestedTemplateDir, "boilerplate.yml"), []byte(`dependencies:
  - name: child-dependency
    template-url: ./child-template
    for_each:
      - child1
      - child2
      - child3
    output-folder: "child/{{ .__each__ }}"
variables:
  - name: ParentName
    default: "{{ .__each__ }}"
`), 0o644)
	require.NoError(t, err)

	// Create child template directory
	childTemplateDir := path.Join(nestedTemplateDir, "child-template")
	err = os.MkdirAll(childTemplateDir, 0o755)
	require.NoError(t, err)

	// Create simple child template
	err = os.WriteFile(path.Join(childTemplateDir, "boilerplate.yml"), []byte(`variables:
  - name: ChildName
    default: "{{ .__each__ }}"
`), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(path.Join(childTemplateDir, "result.txt"), []byte(`Parent: {{ .ParentName }}
Child: {{ .ChildName }}
`), 0o644)
	require.NoError(t, err)

	return templateDir
} 