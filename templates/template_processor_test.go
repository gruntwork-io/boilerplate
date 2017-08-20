package templates

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"os"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/gruntwork-io/boilerplate/variables"
)

func TestOutPath(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	assert.Nil(t, err, "Couldn't get working directory")

	testCases := []struct {
		file	       string
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
		options := config.BoilerplateOptions{
			TemplateFolder: testCase.templateFolder,
			OutputFolder: testCase.outputFolder,
			NonInteractive: true,
			OnMissingKey: config.ExitWithError,
			OnMissingConfig: config.Exit,
		}
		actual, err := outPath(testCase.file, &options, testCase.variables)
		assert.Nil(t, err, "Got unexpected error (file = %s, templateFolder = %s, outputFolder = %s, and variables = %s): %v", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables, err)
		assert.Equal(t, testCase.expected, actual, "(file = %s, templateFolder = %s, outputFolder = %s, and variables = %s)", testCase.file, testCase.templateFolder, testCase.outputFolder, testCase.variables)
	}
}

func TestCloneOptionsForDependency(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		dependency      variables.Dependency
		options         config.BoilerplateOptions
		variables       map[string]interface{}
		expectedOptions config.BoilerplateOptions
	}{
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			config.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: config.ExitWithError},
			map[string]interface{}{},
			config.BoilerplateOptions{TemplateFolder: "/template/dep1", OutputFolder: "/output/out1", NonInteractive: true, Vars: map[string]interface{}{}, OnMissingKey: config.ExitWithError},
		},
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			config.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{"foo": "bar"}, OnMissingKey: config.Invalid},
			map[string]interface{}{"baz": "blah"},
			config.BoilerplateOptions{TemplateFolder: "/template/dep1", OutputFolder: "/output/out1", NonInteractive: false, Vars: map[string]interface{}{"baz": "blah"}, OnMissingKey: config.Invalid},
		},
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "{{ .foo }}", OutputFolder: "{{ .baz }}"},
			config.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]interface{}{}, OnMissingKey: config.ExitWithError},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			config.BoilerplateOptions{TemplateFolder: "/template/path/bar", OutputFolder: "/output/path/blah", NonInteractive: false, Vars: map[string]interface{}{"foo": "bar", "baz": "blah"}, OnMissingKey: config.ExitWithError},
		},
	}

	for _, testCase := range testCases {
		actualOptions, err := cloneOptionsForDependency(testCase.dependency, &testCase.options, testCase.variables)
		assert.NoError(t, err, "Dependency: %s", testCase.dependency)
		assert.Equal(t, testCase.expectedOptions, *actualOptions, "Dependency: %s", testCase.dependency)
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
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{},
			map[string]interface{}{},
		},
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
			map[string]interface{}{"foo": "bar", "baz": "blah"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified", "abc": "should-be-overwritten-by-dep1.abc"},
			map[string]interface{}{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			variables.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1", DontInheritVariables: true},
			map[string]interface{}{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]interface{}{},
		},
	}

	for _, testCase := range testCases {
		actualVariables := cloneVariablesForDependency(testCase.dependency, testCase.variables)
		assert.Equal(t, testCase.expectedVariables, actualVariables, "Dependency: %s", testCase.dependency)
	}
}