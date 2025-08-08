package render //nolint:testpackage

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/options"
	"github.com/gruntwork-io/boilerplate/testutil"
)

const embedWholeFileTemplate = `
embed file:
{{snippet "../test-fixtures/templates-test/full-file-snippet.txt"}}
`

const embedWholeFileTemplateOutput = `
embed file:
Hi
boilerplate-snippet: foo
Hello, World!
boilerplate-snippet: foo
Bye
`

const embedSnippetTemplate = `
embed snippet:
{{snippet "../test-fixtures/templates-test/full-file-snippet.txt" "foo"}}
`

const embedSnippetTemplateOutput = `
embed snippet:
Hello, World!
`

func TestRenderTemplate(t *testing.T) {
	t.Parallel()

	pwd, err := os.Getwd()
	require.NoError(t, err, "Couldn't get working directory")

	// Read an environment variable that's probably present on all systems so we can check that the env helper
	// returns the same value
	userFromEnvVar := os.Getenv("USER")

	defaultOutputDir := "/output"
	defaultTemplateDir := "/templates"
	if runtime.GOOS == windowsOS {
		defaultOutputDir = "C:\\output"
		defaultTemplateDir = "C:\\templates"
	}

	testCases := []struct {
		templateContents  string
		variables         map[string]any
		missingKeyAction  options.MissingKeyAction
		expectedErrorText string
		expectedOutput    string
		skip              bool // flag to skip tests
	}{
		{templateContents: "", variables: map[string]any{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "", skip: false},
		{templateContents: "plain text template", variables: map[string]any{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "plain text template", skip: false},
		{templateContents: "variable lookup: {{.Foo}}", variables: map[string]any{"Foo": "bar"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "variable lookup: bar", skip: false},
		{templateContents: "missing variable lookup, ExitWithError: {{.Foo}}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "map has no entry for key \"Foo\"", expectedOutput: "", skip: false},
		{templateContents: "missing variable lookup, Invalid: {{.Foo}}", variables: map[string]interface{}{}, missingKeyAction: options.Invalid, expectedErrorText: "", expectedOutput: "missing variable lookup, Invalid: <no value>", skip: false},
		// Note: options.ZeroValue does not work correctly with Go templating when you pass in a map[string]interface{}. For some reason, it always prints <no value>.
		{templateContents: "missing variable lookup, ZeroValue: {{.Foo}}", variables: map[string]interface{}{}, missingKeyAction: options.ZeroValue, expectedErrorText: "", expectedOutput: "missing variable lookup, ZeroValue: <no value>", skip: false},
		{templateContents: embedWholeFileTemplate, variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: embedWholeFileTemplateOutput, skip: runtime.GOOS == "windows"},
		{templateContents: embedSnippetTemplate, variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: embedSnippetTemplateOutput, skip: false},
		{templateContents: "Invalid template syntax: {{.Foo", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "unclosed action", expectedOutput: "", skip: false},
		{templateContents: "Uppercase test: {{ .Foo | upcase }}", variables: map[string]interface{}{"Foo": "some text"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Uppercase test: SOME TEXT", skip: false},
		{templateContents: "Lowercase test: {{ .Foo | downcase }}", variables: map[string]interface{}{"Foo": "SOME TEXT"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Lowercase test: some text", skip: false},
		{templateContents: "Capitalize test: {{ .Foo | capitalize }}", variables: map[string]interface{}{"Foo": "some text"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Capitalize test: Some Text", skip: false},
		{templateContents: "Replace test: {{ .Foo | replace \"foo\" \"bar\" }}", variables: map[string]interface{}{"Foo": "hello foo, how are foo"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Replace test: hello bar, how are foo", skip: false},
		{templateContents: "Replace all test: {{ .Foo | replaceAll \"foo\" \"bar\" }}", variables: map[string]interface{}{"Foo": "hello foo, how are foo"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Replace all test: hello bar, how are bar", skip: false},
		{templateContents: "Trim test: {{ .Foo | trim }}", variables: map[string]interface{}{"Foo": "   some text     \t"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Trim test: some text", skip: false},
		{templateContents: "Round test: {{ .Foo | round }}", variables: map[string]interface{}{"Foo": "0.45"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Round test: 0", skip: false},
		{templateContents: "Ceil test: {{ .Foo | ceil }}", variables: map[string]interface{}{"Foo": "0.45"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Ceil test: 1", skip: false},
		{templateContents: "Floor test: {{ .Foo | floor }}", variables: map[string]interface{}{"Foo": "0.45"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Floor test: 0", skip: false},
		{templateContents: "Dasherize test: {{ .Foo | dasherize }}", variables: map[string]interface{}{"Foo": "foo BAR baz!"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Dasherize test: foo-bar-baz", skip: false},
		{templateContents: "Snake case test: {{ .Foo | snakeCase }}", variables: map[string]interface{}{"Foo": "foo BAR baz!"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Snake case test: foo_bar_baz", skip: false},
		{templateContents: "Camel case test: {{ .Foo | camelCase }}", variables: map[string]interface{}{"Foo": "foo BAR baz!"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Camel case test: FooBARBaz", skip: false},
		{templateContents: "Camel case lower test: {{ .Foo | camelCaseLower }}", variables: map[string]interface{}{"Foo": "foo BAR baz!"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Camel case lower test: fooBARBaz", skip: false},
		{templateContents: "Plus test: {{ plus .Foo .Bar }}", variables: map[string]interface{}{"Foo": "5", "Bar": "3"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Plus test: 8", skip: false},
		{templateContents: "Minus test: {{ minus .Foo .Bar }}", variables: map[string]interface{}{"Foo": "5", "Bar": "3"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Minus test: 2", skip: false},
		{templateContents: "Times test: {{ times .Foo .Bar }}", variables: map[string]interface{}{"Foo": "5", "Bar": "3"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Times test: 15", skip: false},
		{templateContents: "Divide test: {{ divide .Foo .Bar | printf \"%1.5f\" }}", variables: map[string]interface{}{"Foo": "5", "Bar": "3"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Divide test: 1.66667", skip: false},
		{templateContents: "Mod test: {{ mod .Foo .Bar }}", variables: map[string]interface{}{"Foo": "5", "Bar": "3"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Mod test: 2", skip: false},
		{templateContents: "Slice test: {{ slice 0 5 1 }}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Slice test: [0 1 2 3 4]", skip: false},
		{templateContents: "Keys test: {{ keys .Map }}", variables: map[string]interface{}{"Map": map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"}}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Keys test: [key1 key2 key3]", skip: false},
		{templateContents: "Shell test: {{ shell \"echo\" .Text }}", variables: map[string]interface{}{"Text": "Hello, World"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Shell test: Hello, World\n", skip: runtime.GOOS == "windows"},
		{templateContents: "Shell set env vars test: {{ shell \"printenv\" \"FOO\" \"ENV:FOO=bar\" }}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Shell set env vars test: bar\n", skip: runtime.GOOS == "windows"},
		{templateContents: "Shell read env vars test: {{ env \"USER\" \"should-not-get-fallback\" }}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Shell read env vars test: " + userFromEnvVar, skip: runtime.GOOS == "windows"},
		{templateContents: "Shell read env vars test, fallback: {{ env \"not-a-valid-env-var\" \"should-get-fallback\" }}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Shell read env vars test, fallback: should-get-fallback", skip: false},
		{templateContents: "Template folder test: {{ templateFolder }}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Template folder test: " + defaultTemplateDir, skip: false},
		{templateContents: "Output folder test: {{ outputFolder }}", variables: map[string]interface{}{}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Output folder test: " + defaultOutputDir, skip: false},
		{templateContents: "Filter chain test: {{ .Foo | downcase | replaceAll \" \" \"\" }}", variables: map[string]interface{}{"Foo": "foo BAR baz!"}, missingKeyAction: options.ExitWithError, expectedErrorText: "", expectedOutput: "Filter chain test: foobarbaz!", skip: false},
	}

	for _, tc := range testCases {
		t.Run(tc.templateContents, func(t *testing.T) {
			t.Parallel()

			if tc.skip {
				t.Skip("Skipping test because of skip flag")
				return
			}
			opts := testutil.CreateTestOptionsWithOutput("/templates", defaultOutputDir)
			opts.OnMissingKey = tc.missingKeyAction
			actualOutput, err := RenderTemplateFromString(pwd+"/template.txt", tc.templateContents, tc.variables, opts)
			if tc.expectedErrorText == "" {
				assert.NoError(t, err, "template = %s, variables = %s, missingKeyAction = %s, err = %v", tc.templateContents, tc.variables, tc.missingKeyAction, err)
				assert.Equal(t, tc.expectedOutput, actualOutput, "template = %s, variables = %s, missingKeyAction = %s", tc.templateContents, tc.variables, tc.missingKeyAction)
			} else if assert.Error(t, err, "template = %s, variables = %s, missingKeyAction = %s", tc.templateContents, tc.variables, tc.missingKeyAction) {
				assert.Contains(t, err.Error(), tc.expectedErrorText, "template = %s, variables = %s, missingKeyAction = %s", tc.templateContents, tc.variables, tc.missingKeyAction)
			}
		})
	}
}
