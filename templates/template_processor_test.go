package templates

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/variables"
	"github.com/stretchr/testify/assert"
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
