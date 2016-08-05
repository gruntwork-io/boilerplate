package templates

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"os"
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
		expectedErrorText string
		expectedOutput    string
	}{
		{"", map[string]string{}, "", ""},
		{"plain text template", map[string]string{}, "", "plain text template"},
		{"variable lookup: {{.Foo}}", map[string]string{"Foo": "bar"}, "", "variable lookup: bar"},
		{"missing variable lookup: {{.Foo}}", map[string]string{}, "", "missing variable lookup: <no value>"},
		{EMBED_WHOLE_FILE_TEMPLATE, map[string]string{}, "", EMBED_WHOLE_FILE_TEMPLATE_OUTPUT},
		{EMBED_SNIPPET_TEMPLATE, map[string]string{}, "", EMBED_SNIPPET_TEMPLATE_OUTPUT},
		{"Invalid template syntax: {{.Foo", map[string]string{}, "unclosed action", ""},
		{"Uppercase test: {{ .Foo | upcase }}", map[string]string{"Foo": "some text"}, "", "Uppercase test: SOME TEXT"},
		{"Lowercase test: {{ .Foo | downcase }}", map[string]string{"Foo": "SOME TEXT"}, "", "Lowercase test: some text"},
		{"Capitalize test: {{ .Foo | capitalize }}", map[string]string{"Foo": "some text"}, "", "Capitalize test: Some Text"},
		{"Replace test: {{ .Foo | replace \"foo\" \"bar\" }}", map[string]string{"Foo": "hello foo, how are foo"}, "", "Replace test: hello bar, how are foo"},
		{"Replace all test: {{ .Foo | replaceAll \"foo\" \"bar\" }}", map[string]string{"Foo": "hello foo, how are foo"}, "", "Replace all test: hello bar, how are bar"},
		{"Trim test: {{ .Foo | trim }}", map[string]string{"Foo": "   some text     \t"}, "", "Trim test: some text"},
		{"Round test: {{ .Foo | round }}", map[string]string{"Foo": "0.45"}, "", "Round test: 0"},
		{"Ceil test: {{ .Foo | ceil }}", map[string]string{"Foo": "0.45"}, "", "Ceil test: 1"},
		{"Floor test: {{ .Foo | floor }}", map[string]string{"Foo": "0.45"}, "", "Floor test: 0"},
		{"Dasherize test: {{ .Foo | dasherize }}", map[string]string{"Foo": "foo BAR baz!"}, "", "Dasherize test: foo-bar-baz"},
		{"Snake case test: {{ .Foo | snakeCase }}", map[string]string{"Foo": "foo BAR baz!"}, "", "Snake case test: foo_bar_baz"},
		{"Camel case test: {{ .Foo | camelCase }}", map[string]string{"Foo": "foo BAR baz!"}, "", "Camel case test: FooBARBaz"},
		{"Camel case lower test: {{ .Foo | camelCaseLower }}", map[string]string{"Foo": "foo BAR baz!"}, "", "Camel case lower test: fooBARBaz"},
	}

	for _, testCase := range testCases {
		actualOutput, err := renderTemplate(pwd + "/template.txt", testCase.templateContents, testCase.variables)
		if testCase.expectedErrorText == "" {
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedOutput, actualOutput)
		} else {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), testCase.expectedErrorText)
		}
	}
}