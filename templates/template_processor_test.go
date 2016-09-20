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

const EMBED_WHOLE_FILE_TEMPLATE =
`
embed file:
{{snippet "../test-fixtures/templates-test/full-file-snippet.txt"}}
`

const EMBED_WHOLE_FILE_TEMPLATE_OUTPUT =
`
embed file:
Hi
boilerplate-snippet: foo
Hello, World!
boilerplate-snippet: foo
Bye
`

const EMBED_SNIPPET_TEMPLATE =
`
embed snippet:
{{snippet "../test-fixtures/templates-test/full-file-snippet.txt" "foo"}}
`

const EMBED_SNIPPET_TEMPLATE_OUTPUT =
`
embed snippet:
Hello, World!
`

func TestRenderTemplate(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	assert.Nil(t, err, "Couldn't get working directory")

	testCases := []struct {
		templateContents  string
		variables   	  map[string]interface{}
		missingKeyAction  config.MissingKeyAction
		expectedErrorText string
		expectedOutput    string
	}{
		{"", map[string]interface{}{}, config.ExitWithError, "", ""},
		{"plain text template", map[string]interface{}{}, config.ExitWithError, "", "plain text template"},
		{"variable lookup: {{.Foo}}", map[string]interface{}{"Foo": "bar"}, config.ExitWithError, "", "variable lookup: bar"},
		{"missing variable lookup, ExitWithError: {{.Foo}}", map[string]interface{}{}, config.ExitWithError, "map has no entry for key \"Foo\"", ""},
		{"missing variable lookup, Invalid: {{.Foo}}", map[string]interface{}{}, config.Invalid, "", "missing variable lookup, Invalid: <no value>"},
		// Note: config.ZeroValue does not work correctly with Go templating when you pass in a map[string]interface{}. For some reason, it always prints <no value>.
		{"missing variable lookup, ZeroValue: {{.Foo}}", map[string]interface{}{}, config.ZeroValue, "", "missing variable lookup, ZeroValue: <no value>"},
		{EMBED_WHOLE_FILE_TEMPLATE, map[string]interface{}{}, config.ExitWithError, "", EMBED_WHOLE_FILE_TEMPLATE_OUTPUT},
		{EMBED_SNIPPET_TEMPLATE, map[string]interface{}{}, config.ExitWithError, "", EMBED_SNIPPET_TEMPLATE_OUTPUT},
		{"Invalid template syntax: {{.Foo", map[string]interface{}{}, config.ExitWithError, "unclosed action", ""},
		{"Uppercase test: {{ .Foo | upcase }}", map[string]interface{}{"Foo": "some text"}, config.ExitWithError, "", "Uppercase test: SOME TEXT"},
		{"Lowercase test: {{ .Foo | downcase }}", map[string]interface{}{"Foo": "SOME TEXT"}, config.ExitWithError, "", "Lowercase test: some text"},
		{"Capitalize test: {{ .Foo | capitalize }}", map[string]interface{}{"Foo": "some text"}, config.ExitWithError, "", "Capitalize test: Some Text"},
		{"Replace test: {{ .Foo | replace \"foo\" \"bar\" }}", map[string]interface{}{"Foo": "hello foo, how are foo"}, config.ExitWithError, "", "Replace test: hello bar, how are foo"},
		{"Replace all test: {{ .Foo | replaceAll \"foo\" \"bar\" }}", map[string]interface{}{"Foo": "hello foo, how are foo"}, config.ExitWithError, "", "Replace all test: hello bar, how are bar"},
		{"Trim test: {{ .Foo | trim }}", map[string]interface{}{"Foo": "   some text     \t"}, config.ExitWithError, "", "Trim test: some text"},
		{"Round test: {{ .Foo | round }}", map[string]interface{}{"Foo": "0.45"}, config.ExitWithError, "", "Round test: 0"},
		{"Ceil test: {{ .Foo | ceil }}", map[string]interface{}{"Foo": "0.45"}, config.ExitWithError, "", "Ceil test: 1"},
		{"Floor test: {{ .Foo | floor }}", map[string]interface{}{"Foo": "0.45"}, config.ExitWithError, "", "Floor test: 0"},
		{"Dasherize test: {{ .Foo | dasherize }}", map[string]interface{}{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Dasherize test: foo-bar-baz"},
		{"Snake case test: {{ .Foo | snakeCase }}", map[string]interface{}{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Snake case test: foo_bar_baz"},
		{"Camel case test: {{ .Foo | camelCase }}", map[string]interface{}{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Camel case test: FooBARBaz"},
		{"Camel case lower test: {{ .Foo | camelCaseLower }}", map[string]interface{}{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Camel case lower test: fooBARBaz"},
		{"Plus test: {{ plus .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Plus test: 8"},
		{"Minus test: {{ minus .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Minus test: 2"},
		{"Times test: {{ times .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Times test: 15"},
		{"Divide test: {{ divide .Foo .Bar | printf \"%1.5f\" }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Divide test: 1.66667"},
		{"Mod test: {{ mod .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Mod test: 2"},
		{"Slice test: {{ slice 0 5 1 }}", map[string]interface{}{}, config.ExitWithError, "", "Slice test: [0 1 2 3 4]"},
		{"Keys test: {{ keys .Map }}", map[string]interface{}{"Map": map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}}, config.ExitWithError, "", "Keys test: [key1 key2 key3]"},
		{"Filter chain test: {{ .Foo | downcase | replaceAll \" \" \"\" }}", map[string]interface{}{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Filter chain test: foobarbaz!"},
	}

	for _, testCase := range testCases {
		actualOutput, err := renderTemplate(pwd + "/template.txt", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
		if testCase.expectedErrorText == "" {
			assert.Nil(t, err, "template = %s, variables = %s, missingKeyAction = %s, err = %v", testCase.templateContents, testCase.variables, testCase.missingKeyAction, err)
			assert.Equal(t, testCase.expectedOutput, actualOutput, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
		} else {
			assert.NotNil(t, err, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
			assert.Contains(t, err.Error(), testCase.expectedErrorText, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
		}
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
	}

	for _, testCase := range testCases {
		actualOptions := cloneOptionsForDependency(testCase.dependency, &testCase.options, testCase.variables)
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