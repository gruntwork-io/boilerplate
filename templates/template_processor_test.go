package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutPath(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	assert.NoError(t, err)

	testCases := []struct {
		file           string
		templateFolder string
		outputFolder   string
		variables      map[string]interface{}
		expected       string
	}{
		{"template-folder/foo.txt", "template-folder", "output-folder", map[string]interface{}{}, "output-folder/foo.txt"},
		{"foo/bar/template-folder/foo.txt", "foo/bar/template-folder", "output-folder", map[string]interface{}{}, "output-folder/foo.txt"},
		{"template-folder/foo.txt", pwd + "/template-folder", "output-folder", map[string]interface{}{}, "output-folder/foo.txt"},
		{"template-folder/foo/bar/baz.txt", pwd + "/template-folder", "output-folder", map[string]interface{}{}, "output-folder/foo/bar/baz.txt"},
		{"template-folder/{{.Foo}}.txt", pwd + "/template-folder", "output-folder", map[string]interface{}{"Foo": "foo"}, "output-folder/foo.txt"},
		{"template-folder/{{.Foo | dasherize}}.txt", pwd + "/template-folder", "output-folder", map[string]interface{}{"Foo": "Foo Bar Baz"}, "output-folder/foo-bar-baz.txt"},
	}

	for _, testCase := range testCases {
		opts := options.BoilerplateOptions{
			TemplateFolder:  testCase.templateFolder,
			OutputFolder:    testCase.outputFolder,
			NonInteractive:  true,
			OnMissingKey:    options.ExitWithError,
			OnMissingConfig: options.Exit,
		}
		actual, err := outPath(testCase.file, &opts, testCase.variables)
		assert.NoError(t, err, "Got unexpected error (file = %s, templateFolder = %s, outputFolder = %s, and variables = %s): %v", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables, err)
		assert.Equal(t, filepath.FromSlash(testCase.expected), filepath.FromSlash(actual), "(file = %s, templateFolder = %s, outputFolder = %s, and variables = %s)", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables)
	}
}

func TestCloneOptionsForDependency(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dependency   variables.Dependency
		opts         options.BoilerplateOptions
		variables    map[string]interface{}
		expectedOpts options.BoilerplateOptions
	}{
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			map[string]interface{}{},
			options.BoilerplateOptions{TemplateUrl: "../dep1", TemplateFolder: filepath.FromSlash("/template/dep1"), OutputFolder: filepath.FromSlash("/output/out1"), NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", DisableDependencyPrompt: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			map[string]interface{}{},
			options.BoilerplateOptions{TemplateUrl: "../dep1", TemplateFolder: filepath.FromSlash("/template/dep1"), OutputFolder: filepath.FromSlash("/output/out1"), DisableDependencyPrompt: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{"foo": "bar"}, OnMissingKey: options.Invalid},
			map[string]interface{}{"baz": "blah"},
			options.BoilerplateOptions{TemplateUrl: "../dep1", TemplateFolder: filepath.FromSlash("/template/dep1"), OutputFolder: filepath.FromSlash("/output/out1"), NonInteractive: false, Vars: map[string]interface{}{"foo": "bar", "baz": "blah"}, OnMissingKey: options.Invalid},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "{{ .foo }}", OutputFolder: "{{ .baz }}"},
			options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			options.BoilerplateOptions{TemplateUrl: "bar", TemplateFolder: filepath.FromSlash("/template/path/bar"), OutputFolder: filepath.FromSlash("/output/path/blah"), NonInteractive: false, Vars: map[string]interface{}{"foo": "bar", "baz": "blah"}, OnMissingKey: options.ExitWithError},
		},
	}

	for _, testCase := range testCases {
		tt := testCase
		t.Run(tt.dependency.Name, func(t *testing.T) {
			actualOptions, err := cloneOptionsForDependency(tt.dependency, &tt.opts, nil, tt.variables)
			assert.NoError(t, err, "Dependency: %s", tt.dependency)
			assert.Equal(t, tt.expectedOpts, *actualOptions, "Dependency: %s", tt.dependency)
		})

	}
}

func TestCloneVariablesForDependency(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dependency        variables.Dependency
		variables         map[string]interface{}
		optsVars          map[string]interface{}
		expectedVariables map[string]interface{}
	}{
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{},
			map[string]interface{}{},
			map[string]interface{}{},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			map[string]interface{}{},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			map[string]interface{}{"dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			map[string]interface{}{"dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified", "abc": "should-be-overwritten-by-dep1.abc"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1", DontInheritVariables: true},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			map[string]interface{}{"dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]interface{}{},
		},
	}

	for _, testCase := range testCases {
		tt := testCase
		t.Run(tt.dependency.Name, func(t *testing.T) {
			opts := &options.BoilerplateOptions{
				TemplateFolder: "/template/path/",
				OutputFolder:   "/output/path/",
				NonInteractive: true,
				Vars:           tt.optsVars,
			}
			actualVariables, err := cloneVariablesForDependency(opts, tt.dependency, nil, tt.variables, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedVariables, actualVariables, "Dependency: %s", tt.dependency)
		})

	}
}

func TestForEachReferenceRendersAsTemplate(t *testing.T) {
	t.Parallel()

	// Test that ForEachReference templates get rendered to resolve variable names dynamically
	tempDir, err := os.MkdirTemp("", "boilerplate-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	templateFolder := filepath.Join(tempDir, "template")
	err = os.MkdirAll(templateFolder, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(templateFolder, "boilerplate.yml"), []byte("variables: []\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(templateFolder, "test.txt"), []byte("{{ .__each__ }}"), 0o644)
	require.NoError(t, err)

	dependency := variables.Dependency{
		Name:         "test",
		TemplateUrl:  ".",
		OutputFolder: "{{ .__each__ }}",
		// This template should render to "template1" by looking up deployments[region_1].template
		ForEachReference: "{{ index .deployments .region \"template\" }}",
	}

	// Test data: region_1 points to template1, which contains the list to iterate over
	variables := map[string]interface{}{
		"region": "region_1",
		"deployments": map[string]interface{}{
			"region_1": map[string]interface{}{
				"template": "template1", // Points to the variable name to use for iteration
			},
		},
		"template1": []string{"a", "b"}, // The actual list that gets iterated over
	}

	opts := &options.BoilerplateOptions{
		TemplateFolder:          templateFolder,
		OutputFolder:            tempDir,
		NonInteractive:          true,
		OnMissingKey:            options.ExitWithError,
		OnMissingConfig:         options.Exit,
		DisableHooks:            true,
		DisableDependencyPrompt: true,
	}

	err = processDependency(dependency, opts, nil, variables)
	require.NoError(t, err)

	// Should create directories "a" and "b" from template1 list
	for _, expected := range []string{"a", "b"} {
		_, err := os.Stat(filepath.Join(tempDir, expected))
		assert.NoError(t, err)
	}
}

// Test both parallel and sequential processing modes, including __each__ variable handling
func TestProcessDependencyBothModes(t *testing.T) {
	tempDir, templateDir := createTestTemplate(t, "boilerplate-both-modes-test")
	defer os.RemoveAll(tempDir)

	dependency := variables.Dependency{
		Name:         "test-dependency",
		TemplateUrl:  templateDir,
		OutputFolder: "output/{{ .__each__ }}",
		ForEach:      []string{"alpha", "beta", "gamma"},
	}

	// Test both parallel and sequential modes
	for _, parallel := range []bool{false, true} {
		mode := "sequential"
		if parallel {
			mode = "parallel"
		}
		
		t.Run(mode, func(t *testing.T) {
			outputDir := filepath.Join(tempDir, mode)
			err := os.MkdirAll(outputDir, 0o755)
			require.NoError(t, err)

			opts := &options.BoilerplateOptions{
				TemplateFolder:  templateDir,
				OutputFolder:    outputDir,
				NonInteractive:  true,
				ParallelForEach: parallel,
				OnMissingKey:    options.ExitWithError,
			}

			err = processDependency(dependency, opts, map[string]variables.Variable{}, map[string]interface{}{})
			require.NoError(t, err)

			// Verify all outputs were created correctly
			for _, item := range dependency.ForEach {
				outputPath := filepath.Join(outputDir, "output", item, "test.txt")
				assert.FileExists(t, outputPath)
				
				content, err := os.ReadFile(outputPath)
				require.NoError(t, err)
				
				// Verify __each__ variable was set correctly
				assert.Contains(t, string(content), fmt.Sprintf("Value: %s", item))
			}
		})
	}
}

// Test error collection in parallel processing
func TestProcessDependencyParallelErrorCollection(t *testing.T) {
	opts := &options.BoilerplateOptions{
		TemplateFolder:  "/nonexistent",
		OutputFolder:    "/tmp",
		NonInteractive:  true,
		ParallelForEach: true,
		OnMissingKey:    options.ExitWithError,
	}

	dependency := variables.Dependency{
		Name:         "failing-dependency",
		TemplateUrl:  "/nonexistent/template",
		OutputFolder: "output/{{ .__each__ }}",
		ForEach:      []string{"item1", "item2", "item3"},
	}

	err := processDependency(dependency, opts, map[string]variables.Variable{}, map[string]interface{}{})
	
	assert.Error(t, err)
	
	// Check if it's a multierror (parallel processing should collect multiple errors)
	if multiErr, ok := err.(*multierror.Error); ok {
		assert.GreaterOrEqual(t, len(multiErr.Errors), 1)
	}
}

// Test that race conditions don't occur during parallel processing
func TestProcessDependencyNoRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	tempDir, templateDir := createTestTemplate(t, "boilerplate-race-test")
	defer os.RemoveAll(tempDir)

	// Run multiple concurrent dependency processes
	numConcurrentRuns := 5
	var wg sync.WaitGroup
	var errorCount int32

	for i := 0; i < numConcurrentRuns; i++ {
		wg.Add(1)
		go func(runIndex int) {
			defer wg.Done()

			runOutputDir := filepath.Join(tempDir, fmt.Sprintf("run%d", runIndex))
			err := os.MkdirAll(runOutputDir, 0o755)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			opts := &options.BoilerplateOptions{
				TemplateFolder:  templateDir,
				OutputFolder:    runOutputDir,
				NonInteractive:  true,
				ParallelForEach: true,
				OnMissingKey:    options.ExitWithError,
			}

			dependency := variables.Dependency{
				Name:         fmt.Sprintf("test-dependency-%d", runIndex),
				TemplateUrl:  templateDir,
				OutputFolder: "output/{{ .__each__ }}",
				ForEach:      []string{"item1", "item2", "item3"},
			}

			if err := processDependency(dependency, opts, map[string]variables.Variable{}, map[string]interface{}{}); err != nil {
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()
	assert.Equal(t, int32(0), errorCount, "Race conditions should not cause errors")
}

// Helper function to create a basic test template
func createTestTemplate(t *testing.T, prefix string) (tempDir, templateDir string) {
	tempDir, err := os.MkdirTemp("", prefix)
	require.NoError(t, err)

	templateDir = filepath.Join(tempDir, "template")
	err = os.MkdirAll(templateDir, 0o755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(templateDir, "boilerplate.yml"), []byte(`variables:
  - name: TestVar
    default: "{{ .__each__ }}"
`), 0o644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(templateDir, "test.txt"), []byte(`Value: {{ .TestVar }}`), 0o644)
	require.NoError(t, err)

	return tempDir, templateDir
}


