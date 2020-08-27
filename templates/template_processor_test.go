package templates

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/variables"
)

func TestOutPath(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	assert.Nil(t, err, "Couldn't get working directory")

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
		assert.Nil(t, err, "Got unexpected error (file = %s, templateFolder = %s, outputFolder = %s, and variables = %s): %v", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables, err)
		assert.Equal(t, testCase.expected, actual, "(file = %s, templateFolder = %s, outputFolder = %s, and variables = %s)", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables)
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
			options.BoilerplateOptions{TemplateUrl: "../dep1", TemplateFolder: "/template/dep1", OutputFolder: "/output/out1", NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{"foo": "bar"}, OnMissingKey: options.Invalid},
			map[string]interface{}{"baz": "blah"},
			options.BoilerplateOptions{TemplateUrl: "../dep1", TemplateFolder: "/template/dep1", OutputFolder: "/output/out1", NonInteractive: false, Vars: map[string]interface{}{"baz": "blah"}, OnMissingKey: options.Invalid},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "{{ .foo }}", OutputFolder: "{{ .baz }}"},
			options.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{}, OnMissingKey: options.ExitWithError},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			options.BoilerplateOptions{TemplateUrl: "bar", TemplateFolder: "/template/path/bar", OutputFolder: "/output/path/blah", NonInteractive: false, Vars: map[string]interface{}{"foo": "bar", "baz": "blah"}, OnMissingKey: options.ExitWithError},
		},
	}

	for _, testCase := range testCases {
		actualOptions, err := cloneOptionsForDependency(testCase.dependency, &testCase.opts, testCase.variables)
		assert.NoError(t, err, "Dependency: %s", testCase.dependency)
		assert.Equal(t, testCase.expectedOpts, *actualOptions, "Dependency: %s", testCase.dependency)
	}
}

func TestCloneVariablesForDependency(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dependency        variables.Dependency
		variables         map[string]interface{}
		expectedVariables map[string]interface{}
	}{
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{},
			map[string]interface{}{},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified", "abc": "should-be-overwritten-by-dep1.abc"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateUrl: "../dep1", OutputFolder: "../out1", DontInheritVariables: true},
			map[string]interface{}{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]interface{}{},
		},
	}

	for _, testCase := range testCases {
		actualVariables := cloneVariablesForDependency(testCase.dependency, testCase.variables)
		assert.Equal(t, testCase.expectedVariables, actualVariables, "Dependency: %s", testCase.dependency)
	}
}
