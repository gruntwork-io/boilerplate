package templates

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"os"
	"github.com/gruntwork-io/boilerplate/config"
)

func TestOutPath(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	assert.Nil(t, err, "Couldn't get working directory")

	testCases := []struct {
		file	       string
		templateFolder string
		outputFolder   string
		expected       string
	}{
		{"template-folder/foo.txt", "template-folder", "output-folder", "output-folder/foo.txt"},
		{"foo/bar/template-folder/foo.txt", "foo/bar/template-folder", "output-folder", "output-folder/foo.txt"},
		{"template-folder/foo.txt", pwd + "/template-folder", "output-folder", "output-folder/foo.txt"},
		{"template-folder/foo/bar/baz.txt", pwd + "/template-folder", "output-folder", "output-folder/foo/bar/baz.txt"},
	}

	for _, testCase := range testCases {
		actual, err := outPath(testCase.file, testCase.templateFolder, testCase.outputFolder)
		assert.Nil(t, err)
		assert.Equal(t, testCase.expected, actual)
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
		variables   	  map[string]string
		missingKeyAction  config.MissingKeyAction
		expectedErrorText string
		expectedOutput    string
	}{
		{"", map[string]string{}, config.ExitWithError, "", ""},
		{"plain text template", map[string]string{}, config.ExitWithError, "", "plain text template"},
		{"variable lookup: {{.Foo}}", map[string]string{"Foo": "bar"}, config.ExitWithError, "", "variable lookup: bar"},
		{"missing variable lookup, ExitWithError: {{.Foo}}", map[string]string{}, config.ExitWithError, "map has no entry for key \"Foo\"", ""},
		{"missing variable lookup, Invalid: {{.Foo}}", map[string]string{}, config.Invalid, "", "missing variable lookup, Invalid: <no value>"},
		{"missing variable lookup, ZeroValue: {{.Foo}}", map[string]string{}, config.ZeroValue, "", "missing variable lookup, ZeroValue: "},
		{EMBED_WHOLE_FILE_TEMPLATE, map[string]string{}, config.ExitWithError, "", EMBED_WHOLE_FILE_TEMPLATE_OUTPUT},
		{EMBED_SNIPPET_TEMPLATE, map[string]string{}, config.ExitWithError, "", EMBED_SNIPPET_TEMPLATE_OUTPUT},
		{"Invalid template syntax: {{.Foo", map[string]string{}, config.ExitWithError, "unclosed action", ""},
		{"Uppercase test: {{ .Foo | upcase }}", map[string]string{"Foo": "some text"}, config.ExitWithError, "", "Uppercase test: SOME TEXT"},
		{"Lowercase test: {{ .Foo | downcase }}", map[string]string{"Foo": "SOME TEXT"}, config.ExitWithError, "", "Lowercase test: some text"},
		{"Capitalize test: {{ .Foo | capitalize }}", map[string]string{"Foo": "some text"}, config.ExitWithError, "", "Capitalize test: Some Text"},
		{"Replace test: {{ .Foo | replace \"foo\" \"bar\" }}", map[string]string{"Foo": "hello foo, how are foo"}, config.ExitWithError, "", "Replace test: hello bar, how are foo"},
		{"Replace all test: {{ .Foo | replaceAll \"foo\" \"bar\" }}", map[string]string{"Foo": "hello foo, how are foo"}, config.ExitWithError, "", "Replace all test: hello bar, how are bar"},
		{"Trim test: {{ .Foo | trim }}", map[string]string{"Foo": "   some text     \t"}, config.ExitWithError, "", "Trim test: some text"},
		{"Round test: {{ .Foo | round }}", map[string]string{"Foo": "0.45"}, config.ExitWithError, "", "Round test: 0"},
		{"Ceil test: {{ .Foo | ceil }}", map[string]string{"Foo": "0.45"}, config.ExitWithError, "", "Ceil test: 1"},
		{"Floor test: {{ .Foo | floor }}", map[string]string{"Foo": "0.45"}, config.ExitWithError, "", "Floor test: 0"},
		{"Dasherize test: {{ .Foo | dasherize }}", map[string]string{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Dasherize test: foo-bar-baz"},
		{"Snake case test: {{ .Foo | snakeCase }}", map[string]string{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Snake case test: foo_bar_baz"},
		{"Camel case test: {{ .Foo | camelCase }}", map[string]string{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Camel case test: FooBARBaz"},
		{"Camel case lower test: {{ .Foo | camelCaseLower }}", map[string]string{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Camel case lower test: fooBARBaz"},
		{"Plus test: {{ plus .Foo .Bar }}", map[string]string{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Plus test: 8"},
		{"Minus test: {{ minus .Foo .Bar }}", map[string]string{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Minus test: 2"},
		{"Times test: {{ times .Foo .Bar }}", map[string]string{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Times test: 15"},
		{"Divide test: {{ divide .Foo .Bar | printf \"%1.5f\" }}", map[string]string{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Divide test: 1.66667"},
		{"Mod test: {{ mod .Foo .Bar }}", map[string]string{"Foo": "5", "Bar": "3"}, config.ExitWithError, "", "Mod test: 2"},
		{"Slice test: {{ slice 0 5 1 }}", map[string]string{}, config.ExitWithError, "", "Slice test: [0 1 2 3 4]"},
		{"Filter chain test: {{ .Foo | downcase | replaceAll \" \" \"\" }}", map[string]string{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Filter chain test: foobarbaz!"},
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
		dependency      config.Dependency
		options         config.BoilerplateOptions
		variables       map[string]string
		expectedOptions config.BoilerplateOptions
	}{
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			config.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: true, Vars: map[string]string{}, OnMissingKey: config.ExitWithError},
			map[string]string{},
			config.BoilerplateOptions{TemplateFolder: "/template/dep1", OutputFolder: "/output/out1", NonInteractive: true, Vars: map[string]string{}, OnMissingKey: config.ExitWithError},
		},
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			config.BoilerplateOptions{TemplateFolder: "/template/path/", OutputFolder: "/output/path/", NonInteractive: false, Vars: map[string]string{"foo": "bar"}, OnMissingKey: config.Invalid},
			map[string]string{"baz": "blah"},
			config.BoilerplateOptions{TemplateFolder: "/template/dep1", OutputFolder: "/output/out1", NonInteractive: false, Vars: map[string]string{"baz": "blah"}, OnMissingKey: config.Invalid},
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
		dependency        config.Dependency
		variables         map[string]string
		expectedVariables map[string]string
	}{
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]string{},
			map[string]string{},
		},
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]string{"foo": "bar", "baz": "blah"},
			map[string]string{"foo": "bar", "baz": "blah"},
		},
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]string{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]string{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1"},
			map[string]string{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified", "abc": "should-be-overwritten-by-dep1.abc"},
			map[string]string{"foo": "bar", "baz": "blah", "abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
		},
		{
			config.Dependency{Name: "dep1", TemplateFolder: "../dep1", OutputFolder: "../out1", DontInheritVariables: true},
			map[string]string{"foo": "bar", "baz": "blah", "dep1.abc": "should-modify-name", "dep2.def": "should-copy-unmodified"},
			map[string]string{},
		},
	}

	for _, testCase := range testCases {
		actualVariables := cloneVariablesForDependency(testCase.dependency, testCase.variables)
		assert.Equal(t, testCase.expectedVariables, actualVariables, "Dependency: %s", testCase.dependency)
	}
}