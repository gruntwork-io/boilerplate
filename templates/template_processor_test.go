package templates //nolint:testpackage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/testutil"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutPath(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	require.NoError(t, err)

	testCases := []struct {
		file           string
		templateFolder string
		outputFolder   string
		variables      map[string]any
		expected       string
	}{
		{file: "template-folder/foo.txt", templateFolder: "template-folder", outputFolder: "output-folder", variables: map[string]any{}, expected: "output-folder/foo.txt"},
		{file: "foo/bar/template-folder/foo.txt", templateFolder: "foo/bar/template-folder", outputFolder: "output-folder", variables: map[string]any{}, expected: "output-folder/foo.txt"},
		{file: "template-folder/foo.txt", templateFolder: pwd + "/template-folder", outputFolder: "output-folder", variables: map[string]any{}, expected: "output-folder/foo.txt"},
		{file: "template-folder/foo/bar/baz.txt", templateFolder: pwd + "/template-folder", outputFolder: "output-folder", variables: map[string]any{}, expected: "output-folder/foo/bar/baz.txt"},
		{file: "template-folder/{{.Foo}}.txt", templateFolder: pwd + "/template-folder", outputFolder: "output-folder", variables: map[string]interface{}{"Foo": "foo"}, expected: "output-folder/foo.txt"},
		{file: "template-folder/{{.Foo | dasherize}}.txt", templateFolder: pwd + "/template-folder", outputFolder: "output-folder", variables: map[string]interface{}{"Foo": "Foo Bar Baz"}, expected: "output-folder/foo-bar-baz.txt"},
	}

	for _, testCase := range testCases {
		opts := options.BoilerplateOptions{
			TemplateFolder:  testCase.templateFolder,
			OutputFolder:    testCase.outputFolder,
			NonInteractive:  true,
			OnMissingKey:    options.ExitWithError,
			OnMissingConfig: options.Exit,
		}
		actual, err := outPath(t.Context(), testCase.file, &opts, testCase.variables)
		require.NoError(t, err, "Got unexpected error (file = %s, templateFolder = %s, outputFolder = %s, and variables = %s): %v", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables, err)
		assert.Equal(t, filepath.FromSlash(testCase.expected), filepath.FromSlash(actual), "(file = %s, templateFolder = %s, outputFolder = %s, and variables = %s)", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables)
	}
}

func TestCloneOptionsForDependency(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		variables    map[string]interface{}
		opts         options.BoilerplateOptions
		expectedOpts options.BoilerplateOptions
		dependency   variables.Dependency
	}{
		{
			dependency:   variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			opts:         options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			variables:    map[string]interface{}{},
			expectedOpts: options.BoilerplateOptions{TemplateURL: "../dep1", TemplateFolder: filepath.FromSlash("/template/dep1"), OutputFolder: filepath.FromSlash("/output/out1"), NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
		},
		{
			dependency:   variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			opts:         options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", DisableDependencyPrompt: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			variables:    map[string]interface{}{},
			expectedOpts: options.BoilerplateOptions{TemplateURL: "../dep1", TemplateFolder: filepath.FromSlash("/template/dep1"), OutputFolder: filepath.FromSlash("/output/out1"), DisableDependencyPrompt: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
		},
		{
			dependency:   variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			opts:         options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{"foo": "bar"}, OnMissingKey: options.Invalid},
			variables:    map[string]interface{}{"baz": "blah"},
			expectedOpts: options.BoilerplateOptions{TemplateURL: "../dep1", TemplateFolder: filepath.FromSlash("/template/dep1"), OutputFolder: filepath.FromSlash("/output/out1"), NonInteractive: false, Vars: map[string]interface{}{"foo": "bar", "baz": "blah"}, OnMissingKey: options.Invalid},
		},
		{
			dependency:   variables.Dependency{Name: "dep1", TemplateURL: "{{ .foo }}", OutputFolder: "{{ .baz }}"},
			opts:         options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			variables:    map[string]interface{}{"foo": "bar", "baz": "blah"},
			expectedOpts: options.BoilerplateOptions{TemplateURL: "bar", TemplateFolder: filepath.FromSlash("/template/path/bar"), OutputFolder: filepath.FromSlash("/output/path/blah"), NonInteractive: false, Vars: map[string]interface{}{"foo": "bar", "baz": "blah"}, OnMissingKey: options.ExitWithError},
		},
	}

	for _, testCase := range testCases {
		tt := testCase
		t.Run(tt.dependency.Name, func(t *testing.T) {
			t.Parallel()

			actualOptions, err := cloneOptionsForDependency(t.Context(), tt.dependency, &tt.opts, nil, tt.variables)
			require.NoError(t, err, "Dependency: %s", tt.dependency)
			assert.Equal(t, tt.expectedOpts, *actualOptions, "Dependency: %s", tt.dependency)
		})
	}
}

func TestCloneVariablesForDependency(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		variables         map[string]interface{}
		optsVars          map[string]interface{}
		expectedVariables map[string]interface{}
		dependency        variables.Dependency
	}{
		{
			dependency:        variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			variables:         map[string]interface{}{},
			optsVars:          map[string]interface{}{},
			expectedVariables: map[string]interface{}{},
		},
		{
			dependency:        variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			variables:         map[string]interface{}{"foo": "bar", "baz": "blah"},
			optsVars:          map[string]interface{}{},
			expectedVariables: map[string]interface{}{"foo": "bar", "baz": "blah"},
		},
		{
			dependency:        variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			variables:         map[string]interface{}{"foo": "bar", "baz": "blah"},
			optsVars:          map[string]interface{}{"dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			expectedVariables: map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			dependency:        variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1"},
			variables:         map[string]interface{}{"foo": "bar", "baz": "blah"},
			optsVars:          map[string]interface{}{"dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified", "abc": "should-be-overwritten-by-dep1.abc"},
			expectedVariables: map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			dependency:        variables.Dependency{Name: "dep1", TemplateURL: "../dep1", OutputFolder: "../out1", DontInheritVariables: true},
			variables:         map[string]interface{}{"foo": "bar", "baz": "blah"},
			optsVars:          map[string]interface{}{"dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			expectedVariables: map[string]interface{}{},
		},
	}

	for _, testCase := range testCases {
		tt := testCase
		t.Run(tt.dependency.Name, func(t *testing.T) {
			t.Parallel()

			opts := testutil.CreateTestOptionsWithOutput("/template/path/", "/output/path/")
			opts.Vars = tt.optsVars
			actualVariables, err := cloneVariablesForDependency(t.Context(), opts, tt.dependency, nil, tt.variables, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedVariables, actualVariables, "Dependency: %s", tt.dependency)
		})
	}
}

func TestForEachReferenceRendersAsTemplate(t *testing.T) {
	t.Parallel()

	// Test that ForEachReference templates get rendered to resolve variable names dynamically
	tempDir := t.TempDir()

	templateFolder := filepath.Join(tempDir, "template")
	err := os.MkdirAll(templateFolder, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(templateFolder, "boilerplate.yml"), []byte("variables: []\n"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(templateFolder, "test.txt"), []byte("{{ .__each__ }}"), 0o644)
	require.NoError(t, err)

	dependency := variables.Dependency{
		Name:         "test",
		TemplateURL:  ".",
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

	opts := testutil.CreateTestOptionsWithOutput(templateFolder, tempDir)

	err = processDependency(t.Context(), dependency, opts, nil, variables)
	require.NoError(t, err)

	// Should create directories "a" and "b" from template1 list
	for _, expected := range []string{"a", "b"} {
		_, err := os.Stat(filepath.Join(tempDir, expected))
		require.NoError(t, err)
	}
}
