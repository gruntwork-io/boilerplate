package render

import (
	"os"
	"fmt"
	"testing"
	"github.com/gruntwork-io/boilerplate/config"
	"github.com/stretchr/testify/assert"
)

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

	// Read an environment variable that's probably present on all systems so we can check that the env helper
	// returns the same value
	userFromEnvVar := os.Getenv("USER")

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
		{"Shell test: {{ shell \"echo\" .Text }}", map[string]interface{}{"Text": "Hello, World"}, config.ExitWithError, "", "Shell test: Hello, World\n"},
		{"Shell set env vars test: {{ shell \"printenv\" \"FOO\" \"ENV:FOO=bar\" }}", map[string]interface{}{}, config.ExitWithError, "", "Shell set env vars test: bar\n"},
		{"Shell read env vars test: {{ env \"USER\" \"should-not-get-fallback\" }}", map[string]interface{}{}, config.ExitWithError, "", fmt.Sprintf("Shell read env vars test: %s", userFromEnvVar)},
		{"Shell read env vars test, fallback: {{ env \"not-a-valid-env-var\" \"should-get-fallback\" }}", map[string]interface{}{}, config.ExitWithError, "", "Shell read env vars test, fallback: should-get-fallback"},
		{"Template folder test: {{ templateFolder }}", map[string]interface{}{}, config.ExitWithError, "", "Template folder test: /templates"},
		{"Output folder test: {{ outputFolder }}", map[string]interface{}{}, config.ExitWithError, "", "Output folder test: /output"},
		{"Filter chain test: {{ .Foo | downcase | replaceAll \" \" \"\" }}", map[string]interface{}{"Foo": "foo BAR baz!"}, config.ExitWithError, "", "Filter chain test: foobarbaz!"},
	}

	for _, testCase := range testCases {
		actualOutput, err := RenderTemplate(pwd + "/template.txt", testCase.templateContents, testCase.variables, &config.BoilerplateOptions{TemplateFolder: "/templates", OutputFolder: "/output", OnMissingKey: testCase.missingKeyAction})
		if testCase.expectedErrorText == "" {
			assert.Nil(t, err, "template = %s, variables = %s, missingKeyAction = %s, err = %v", testCase.templateContents, testCase.variables, testCase.missingKeyAction, err)
			assert.Equal(t, testCase.expectedOutput, actualOutput, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
		} else {
			if assert.NotNil(t, err, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction) {
				assert.Contains(t, err.Error(), testCase.expectedErrorText, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
			}
		}
	}
}

func TestRenderTemplateRecursively(t *testing.T) {
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
		{"Non-recursive test: {{ .Foo }}", map[string]interface{}{"Foo": "foo"}, config.ExitWithError, "", "Non-recursive test: foo"},
		{"Recursive test: {{ .Bar }}", map[string]interface{}{"Foo": "foo", "Bar": "{{ .Foo }}-bar"}, config.ExitWithError, "", "Recursive test: foo-bar"},
		{"Recursive test, multiple levels: {{ .Baz }}", map[string]interface{}{"Foo": "foo", "Bar": "{{ .Foo }}-bar", "Baz": "{{ .Bar }}-baz"}, config.ExitWithError, "", "Recursive test, multiple levels: foo-bar-baz"},
		{"Recursive test, infinite cycle: {{ .Bar }}", map[string]interface{}{"Foo": "{{ .Bar }}", "Bar": "{{ .Foo }}"}, config.ExitWithError, "seems to contain infinite loop", ""},
	}

	for _, testCase := range testCases {
		actualOutput, err := RenderTemplateRecursively(pwd + "/template.txt", testCase.templateContents, testCase.variables, &config.BoilerplateOptions{TemplateFolder: "/templates", OutputFolder: "/output", OnMissingKey: testCase.missingKeyAction})
		if testCase.expectedErrorText == "" {
			assert.Nil(t, err, "template = %s, variables = %s, missingKeyAction = %s, err = %v", testCase.templateContents, testCase.variables, testCase.missingKeyAction, err)
			assert.Equal(t, testCase.expectedOutput, actualOutput, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
		} else {
			if assert.NotNil(t, err, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction) {
				assert.Contains(t, err.Error(), testCase.expectedErrorText, "template = %s, variables = %s, missingKeyAction = %s", testCase.templateContents, testCase.variables, testCase.missingKeyAction)
			}
		}
	}
}

