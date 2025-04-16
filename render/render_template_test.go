package render

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gruntwork-io/boilerplate/options"
)

const EMBED_WHOLE_FILE_TEMPLATE = `
embed file:
{{snippet "../test-fixtures/templates-test/full-file-snippet.txt"}}
`

const EMBED_WHOLE_FILE_TEMPLATE_OUTPUT = `
embed file:
Hi
boilerplate-snippet: foo
Hello, World!
boilerplate-snippet: foo
Bye
`

const EMBED_SNIPPET_TEMPLATE = `
embed snippet:
{{snippet "../test-fixtures/templates-test/full-file-snippet.txt" "foo"}}
`

const EMBED_SNIPPET_TEMPLATE_OUTPUT = `
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

	defaultOutputDir := "/output"
	defaultTemplateDir := "/templates"
	if runtime.GOOS == "windows" {
		defaultOutputDir = "C:\\output"
		defaultTemplateDir = "C:\\templates"
	}

	testCases := []struct {
		templateContents  string
		variables         map[string]interface{}
		missingKeyAction  options.MissingKeyAction
		expectedErrorText string
		expectedOutput    string
		skip              bool // flag to skip tests
	}{
		{"", map[string]interface{}{}, options.ExitWithError, "", "", false},
		{"plain text template", map[string]interface{}{}, options.ExitWithError, "", "plain text template", false},
		{"variable lookup: {{.Foo}}", map[string]interface{}{"Foo": "bar"}, options.ExitWithError, "", "variable lookup: bar", false},
		{"missing variable lookup, ExitWithError: {{.Foo}}", map[string]interface{}{}, options.ExitWithError, "map has no entry for key \"Foo\"", "", false},
		{"missing variable lookup, Invalid: {{.Foo}}", map[string]interface{}{}, options.Invalid, "", "missing variable lookup, Invalid: <no value>", false},
		// Note: options.ZeroValue does not work correctly with Go templating when you pass in a map[string]interface{}. For some reason, it always prints <no value>.
		{"missing variable lookup, ZeroValue: {{.Foo}}", map[string]interface{}{}, options.ZeroValue, "", "missing variable lookup, ZeroValue: <no value>", false},
		{EMBED_WHOLE_FILE_TEMPLATE, map[string]interface{}{}, options.ExitWithError, "", EMBED_WHOLE_FILE_TEMPLATE_OUTPUT, runtime.GOOS == "windows"},
		{EMBED_SNIPPET_TEMPLATE, map[string]interface{}{}, options.ExitWithError, "", EMBED_SNIPPET_TEMPLATE_OUTPUT, false},
		{"Invalid template syntax: {{.Foo", map[string]interface{}{}, options.ExitWithError, "unclosed action", "", false},
		{"Uppercase test: {{ .Foo | upcase }}", map[string]interface{}{"Foo": "some text"}, options.ExitWithError, "", "Uppercase test: SOME TEXT", false},
		{"Lowercase test: {{ .Foo | downcase }}", map[string]interface{}{"Foo": "SOME TEXT"}, options.ExitWithError, "", "Lowercase test: some text", false},
		{"Capitalize test: {{ .Foo | capitalize }}", map[string]interface{}{"Foo": "some text"}, options.ExitWithError, "", "Capitalize test: Some Text", false},
		{"Replace test: {{ .Foo | replace \"foo\" \"bar\" }}", map[string]interface{}{"Foo": "hello foo, how are foo"}, options.ExitWithError, "", "Replace test: hello bar, how are foo", false},
		{"Replace all test: {{ .Foo | replaceAll \"foo\" \"bar\" }}", map[string]interface{}{"Foo": "hello foo, how are foo"}, options.ExitWithError, "", "Replace all test: hello bar, how are bar", false},
		{"Trim test: {{ .Foo | trim }}", map[string]interface{}{"Foo": "   some text     \t"}, options.ExitWithError, "", "Trim test: some text", false},
		{"Round test: {{ .Foo | round }}", map[string]interface{}{"Foo": "0.45"}, options.ExitWithError, "", "Round test: 0", false},
		{"Ceil test: {{ .Foo | ceil }}", map[string]interface{}{"Foo": "0.45"}, options.ExitWithError, "", "Ceil test: 1", false},
		{"Floor test: {{ .Foo | floor }}", map[string]interface{}{"Foo": "0.45"}, options.ExitWithError, "", "Floor test: 0", false},
		{"Dasherize test: {{ .Foo | dasherize }}", map[string]interface{}{"Foo": "foo BAR baz!"}, options.ExitWithError, "", "Dasherize test: foo-bar-baz", false},
		{"Snake case test: {{ .Foo | snakeCase }}", map[string]interface{}{"Foo": "foo BAR baz!"}, options.ExitWithError, "", "Snake case test: foo_bar_baz", false},
		{"Camel case test: {{ .Foo | camelCase }}", map[string]interface{}{"Foo": "foo BAR baz!"}, options.ExitWithError, "", "Camel case test: FooBarBaz", false},
		{"Camel case lower test: {{ .Foo | camelCaseLower }}", map[string]interface{}{"Foo": "foo BAR baz!"}, options.ExitWithError, "", "Camel case lower test: fooBarBaz", false},
		{"Plus test: {{ plus .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, options.ExitWithError, "", "Plus test: 8", false},
		{"Minus test: {{ minus .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, options.ExitWithError, "", "Minus test: 2", false},
		{"Times test: {{ times .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, options.ExitWithError, "", "Times test: 15", false},
		{"Divide test: {{ divide .Foo .Bar | printf \"%1.5f\" }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, options.ExitWithError, "", "Divide test: 1.66667", false},
		{"Mod test: {{ mod .Foo .Bar }}", map[string]interface{}{"Foo": "5", "Bar": "3"}, options.ExitWithError, "", "Mod test: 2", false},
		{"Slice test: {{ slice 0 5 1 }}", map[string]interface{}{}, options.ExitWithError, "", "Slice test: [0 1 2 3 4]", false},
		{"Keys test: {{ keys .Map }}", map[string]interface{}{"Map": map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}}, options.ExitWithError, "", "Keys test: [key1 key2 key3]", false},
		{"Shell test: {{ shell \"echo\" .Text }}", map[string]interface{}{"Text": "Hello, World"}, options.ExitWithError, "", "Shell test: Hello, World\n", runtime.GOOS == "windows"},
		{"Shell set env vars test: {{ shell \"printenv\" \"FOO\" \"ENV:FOO=bar\" }}", map[string]interface{}{}, options.ExitWithError, "", "Shell set env vars test: bar\n", runtime.GOOS == "windows"},
		{"Shell read env vars test: {{ env \"USER\" \"should-not-get-fallback\" }}", map[string]interface{}{}, options.ExitWithError, "", fmt.Sprintf("Shell read env vars test: %s", userFromEnvVar), runtime.GOOS == "windows"},
		{"Shell read env vars test, fallback: {{ env \"not-a-valid-env-var\" \"should-get-fallback\" }}", map[string]interface{}{}, options.ExitWithError, "", "Shell read env vars test, fallback: should-get-fallback", false},
		{"Template folder test: {{ templateFolder }}", map[string]interface{}{}, options.ExitWithError, "", "Template folder test: " + defaultTemplateDir, false},
		{"Output folder test: {{ outputFolder }}", map[string]interface{}{}, options.ExitWithError, "", "Output folder test: " + defaultOutputDir, false},
		{"Filter chain test: {{ .Foo | downcase | replaceAll \" \" \"\" }}", map[string]interface{}{"Foo": "foo BAR baz!"}, options.ExitWithError, "", "Filter chain test: foobarbaz!", false},
	}

	for _, testCase := range testCases {
		tt := testCase
		t.Run(tt.templateContents, func(t *testing.T) {
			if tt.skip {
				t.Skip("Skipping test because of skip flag")
				return
			}
			actualOutput, err := RenderTemplateFromString(pwd+"/template.txt", tt.templateContents, tt.variables, &options.BoilerplateOptions{TemplateFolder: "/templates", OutputFolder: defaultOutputDir, OnMissingKey: testCase.missingKeyAction})
			if tt.expectedErrorText == "" {
				assert.Nil(t, err, "template = %s, variables = %s, missingKeyAction = %s, err = %v", tt.templateContents, tt.variables, tt.missingKeyAction, err)
				assert.Equal(t, tt.expectedOutput, actualOutput, "template = %s, variables = %s, missingKeyAction = %s", tt.templateContents, tt.variables, tt.missingKeyAction)
			} else {
				if assert.NotNil(t, err, "template = %s, variables = %s, missingKeyAction = %s", tt.templateContents, tt.variables, tt.missingKeyAction) {
					assert.Contains(t, err.Error(), tt.expectedErrorText, "template = %s, variables = %s, missingKeyAction = %s", tt.templateContents, tt.variables, tt.missingKeyAction)

				}
			}
		})
	}
}
