package templates

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"reflect"
	"bufio"
	"bytes"
	"github.com/gruntwork-io/boilerplate/errors"
	"fmt"
)

func TestExtractSnippetName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		line            string
		containsSnippet bool
		snippetName     string
	}{
		{"", false, ""},
		{"foo", false, ""},
		{"boilerplate", false, ""},
		{"boilerplate-snippet", false, ""},
		{"boilerplate-snippet:", false, ""},
		{"boilerplate-snippet: ", false, ""},
		{"boilerplate-snippet: foo", true, "foo"},
		{"boilerplate-snippet:foo", true, "foo"},
		{"boilerplate-snippet:\t\tfoo        ", true, "foo"},
		{"<!-- boilerplate-snippet: foo -->", true, "foo"},
		{"// boilerplate-snippet: foo", true, "foo"},
		{"/* boilerplate-snippet: foo */", true, "foo"},
		{"boilerplate-snippet: foo bar", true, "foo"},
		{"boilerplate-snippet:foo-bar-baz", true, "foo-bar-baz"},
	}

	for _, testCase := range testCases {
		snippetName, containsSnippet := extractSnippetName(testCase.line)
		assert.Equal(t, testCase.containsSnippet, containsSnippet)
		assert.Equal(t, testCase.snippetName, snippetName)
	}
}

const MULTILINE_SNIPPET_NOT_TERMINATED =
`
foo
boilerplate-snippet: foo
bar blah
boilerplate-snippet: bar
`

const BODY_TEXT_ONE_LINE = "line1"

const BODY_TEXT_MULTILINE =
`
line1
line2
line3
`

var FULL_FILE_ONE_LINE_SNIPPET = fmt.Sprintf(
`
boilerplate-snippet: foo
%s
boilerplate-snippet: foo
`, BODY_TEXT_ONE_LINE)

var FULL_FILE_MULTILINE_SNIPPET = fmt.Sprintf(
`
boilerplate-snippet: foo
%s
boilerplate-snippet: foo
`, BODY_TEXT_MULTILINE)

var FULL_FILE_MULTILINE_SNIPPET_IN_HTML_COMMENTS = fmt.Sprintf(
`
<!-- boilerplate-snippet: foo -->
%s
<-- boilerplate-snippet: foo -->
`, BODY_TEXT_MULTILINE)

var PARTIAL_FILE_MULTILINE_SNIPPET_IN_C_COMMENTS = fmt.Sprintf(
`
other text
this should be ignored

// boilerplate-snippet: foo
%s
// boilerplate-snippet: foo

this should also
be completely ignored
`, BODY_TEXT_MULTILINE)

var PARTIAL_FILE_ONE_LINE_SNIPPET_IN_MISMATCHED_COMMENTS = fmt.Sprintf(
`
other text
this should be ignored

// boilerplate-snippet: foo
%s
/*boilerplate-snippet:foo */

this should also
be completely ignored
`, BODY_TEXT_ONE_LINE)

var PARTIAL_FILE_MUTLIPLE_SNIPPETS = fmt.Sprintf(
`
other text
this should be ignored

boilerplate-snippet: bar
this should be ignored
boilerplate-snippet: bar

boilerplate-snippet: foo
%s
boilerplate-snippet: foo

boilerplate-snippet: baz
this should also
be completely ignored
boilerplate-snippet: baz
`, BODY_TEXT_ONE_LINE)

var PARTIAL_FILE_EMBEDDED_SNIPPETS = fmt.Sprintf(
`
other text
this should be ignored

boilerplate-snippet: bar
bar

boilerplate-snippet: foo
%s
boilerplate-snippet: foo

blah
boilerplate-snippet: bar

boilerplate-snippet: baz
this should also
be completely ignored
boilerplate-snippet: baz
`, BODY_TEXT_ONE_LINE)

func TestReadSnippetFromScanner(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		text                string
		snippetName         string
		expectedErr	    error
		expectedSnippetText string
	}{
		{"", "foo", SnippetNotFound("foo"), ""},
		{"abcdef", "foo", SnippetNotFound("foo"), ""},
		{"boilerplate-snippet: bar", "foo", SnippetNotFound("foo"), ""},
		{"boilerplate-snippet: foo", "foo", SnippetNotTerminated("foo"), ""},
		{MULTILINE_SNIPPET_NOT_TERMINATED, "foo", SnippetNotTerminated("foo"), ""},
		{FULL_FILE_ONE_LINE_SNIPPET, "foo", nil, BODY_TEXT_ONE_LINE},
		{FULL_FILE_MULTILINE_SNIPPET, "foo", nil, BODY_TEXT_MULTILINE},
		{FULL_FILE_MULTILINE_SNIPPET_IN_HTML_COMMENTS, "foo", nil, BODY_TEXT_MULTILINE},
		{PARTIAL_FILE_MULTILINE_SNIPPET_IN_C_COMMENTS, "foo", nil, BODY_TEXT_MULTILINE},
		{PARTIAL_FILE_ONE_LINE_SNIPPET_IN_MISMATCHED_COMMENTS, "foo", nil, BODY_TEXT_ONE_LINE},
		{PARTIAL_FILE_MUTLIPLE_SNIPPETS, "foo", nil, BODY_TEXT_ONE_LINE},
		{PARTIAL_FILE_EMBEDDED_SNIPPETS, "foo", nil, BODY_TEXT_ONE_LINE},
	}

	for _, testCase := range testCases {
		scanner := bufio.NewScanner(bytes.NewBufferString(testCase.text))
		snippetText, err := readSnippetFromScanner(scanner, testCase.snippetName)

		if testCase.expectedErr == nil {
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedSnippetText, snippetText)
		} else {
			assert.NotNil(t, err)
			assert.True(t, errors.IsError(err, testCase.expectedErr), "Expected %s error but got %s", reflect.TypeOf(testCase.expectedErr), reflect.TypeOf(err))
		}
	}
}

func TestPathRelativeToTemplate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		templatePath string
		path         string
		expected     string
	}{
		{"/template.txt", ".", "/"},
		{"/foo/bar/template.txt", ".", "/foo/bar"},
		{"/foo/bar/template.txt", "..", "/foo"},
		{"/foo/bar/template.txt", "../..", "/"},
		{"/foo/bar/template.txt", "../../bar/baz", "/bar/baz"},
		{"/foo/bar/template.txt", "foo", "/foo/bar/foo"},
		{"/foo/bar/template.txt", "./foo", "/foo/bar/foo"},
		{"/foo/bar/template.txt", "/foo", "/foo"},
		{"/foo/bar/template.txt", "/foo/bar/baz", "/foo/bar/baz"},
	}

	for _, testCase := range testCases {
		actual := pathRelativeToTemplate(testCase.templatePath, testCase.path)
		assert.Equal(t, testCase.expected, actual)
	}
}

func TestWrapWithTemplatePath(t *testing.T) {
	t.Parallel()

	expected := "/foo/bar/template.txt"
	wrappedFunc := wrapWithTemplatePath(expected, func(templatePath string, args ... string) (string, error) {
		return templatePath, nil
	})

	actual, err := wrappedFunc()
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestDasherize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"     ", ""},
		{"foo", "foo"},
		{"FOO", "foo"},
		{" \t  foo   \t", "foo"},
		{"FooBarBaz", "foo-bar-baz"},
		{"Fo", "fo"},
		{"fooID", "foo-id"},
		{"FoB", "fo-b"},
		{"oFo", "o-fo"},
		{"FoBa", "fo-ba"},
		{"oFoBa", "o-fo-ba"},
		{"oFoB", "o-fo-b"},
		{"Foo123B1234Baz1234", "foo123-b1234-baz1234"},
		{"Foo_Bar_Baz", "foo-bar-baz"},
		{"FooIDbarBaz", "foo-idbar-baz"},
		{"FOOIDbarBaz", "fooidbar-baz"},
		{" A B C ", "a-b-c"},
		{"foo bar baz", "foo-bar-baz"},
		{"foo \t \tbar   baz \t", "foo-bar-baz"},
		{"foo_bar_baz", "foo-bar-baz"},
		{"_foo_bar_baz_", "foo-bar-baz"},
		{"foo-bar-baz", "foo-bar-baz"},
		{"foo--bar----baz", "foo-bar-baz"},
		{"foo__bar____baz", "foo-bar-baz"},
		{" Foo Bar Baz ", "foo-bar-baz"},
		{" Foo Bar_BazBlah ", "foo-bar-baz-blah"},
		{" Foo.Bar.Baz", "foo-bar-baz"},
		{"#@!Foo@#$@$Bar>>>>>Baz", "foo-bar-baz"},
	}

	for _, testCase := range testCases {
		actual := dasherize(testCase.input)
		assert.Equal(t, testCase.expected, actual, "When calling dasherize on '%s', expected '%s', but got '%s'", testCase.input, testCase.expected, actual)
	}
}

func TestSnakeCase(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"     ", ""},
		{"foo", "foo"},
		{"FOO", "foo"},
		{" \t  foo   \t", "foo"},
		{"FooBarBaz", "foo_bar_baz"},
		{"Fo", "fo"},
		{"fooID", "foo_id"},
		{"FoB", "fo_b"},
		{"oFo", "o_fo"},
		{"FoBa", "fo_ba"},
		{"oFoBa", "o_fo_ba"},
		{"oFoB", "o_fo_b"},
		{"Foo123B1234Baz1234", "foo123_b1234_baz1234"},
		{"Foo_Bar_Baz", "foo_bar_baz"},
		{"FooIDbarBaz", "foo_idbar_baz"},
		{"FOOIDbarBaz", "fooidbar_baz"},
		{" A B C ", "a_b_c"},
		{"foo bar baz", "foo_bar_baz"},
		{"foo \t \tbar   baz \t", "foo_bar_baz"},
		{"foo_bar_baz", "foo_bar_baz"},
		{"_foo_bar_baz_", "foo_bar_baz"},
		{"foo-bar-baz", "foo_bar_baz"},
		{"foo--bar----baz", "foo_bar_baz"},
		{"foo__bar____baz", "foo_bar_baz"},
		{" Foo Bar Baz ", "foo_bar_baz"},
		{" Foo Bar_BazBlah ", "foo_bar_baz_blah"},
		{" Foo.Bar.Baz", "foo_bar_baz"},
		{"#@!Foo@#$@$Bar>>>>>Baz", "foo_bar_baz"},
	}

	for _, testCase := range testCases {
		actual := snakeCase(testCase.input)
		assert.Equal(t, testCase.expected, actual, "When calling snakeCase on '%s', expected '%s', but got '%s'", testCase.input, testCase.expected, actual)
	}
}

func TestCamelCase(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"     ", ""},
		{"foo", "Foo"},
		{"FOO", "FOO"},
		{" \t  foo   \t", "Foo"},
		{"FooBarBaz", "FooBarBaz"},
		{"Fo", "Fo"},
		{"fooID", "FooID"},
		{"FoB", "FoB"},
		{"oFo", "OFo"},
		{"FoBa", "FoBa"},
		{"oFoBa", "OFoBa"},
		{"oFoB", "OFoB"},
		{"Foo123B1234Baz1234", "Foo123B1234Baz1234"},
		{"Foo_Bar_Baz", "FooBarBaz"},
		{"FooIDbarBaz", "FooIDbarBaz"},
		{"FOOIDbarBaz", "FOOIDbarBaz"},
		{" A B C ", "ABC"},
		{"foo bar baz", "FooBarBaz"},
		{"foo \t \tbar   baz \t", "FooBarBaz"},
		{"foo_bar_baz", "FooBarBaz"},
		{"_foo_bar_baz_", "FooBarBaz"},
		{"foo-bar-baz", "FooBarBaz"},
		{"foo--bar----baz", "FooBarBaz"},
		{"foo__bar____baz", "FooBarBaz"},
		{" Foo Bar Baz ", "FooBarBaz"},
		{" Foo Bar_BazBlah ", "FooBarBazBlah"},
		{" Foo.Bar.Baz", "FooBarBaz"},
		{"#@!Foo@#$@$Bar>>>>>Baz", "FooBarBaz"},
	}

	for _, testCase := range testCases {
		actual := camelCase(testCase.input)
		assert.Equal(t, testCase.expected, actual, "When calling camelCase on '%s', expected '%s', but got '%s'", testCase.input, testCase.expected, actual)
	}
}

func TestCamelCaseLower(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"     ", ""},
		{"foo", "foo"},
		{"FOO", "fOO"},
		{" \t  foo   \t", "foo"},
		{"FooBarBaz", "fooBarBaz"},
		{"Fo", "fo"},
		{"fooID", "fooID"},
		{"FoB", "foB"},
		{"oFo", "oFo"},
		{"FoBa", "foBa"},
		{"oFoBa", "oFoBa"},
		{"oFoB", "oFoB"},
		{"Foo123B1234Baz1234", "foo123B1234Baz1234"},
		{"Foo_Bar_Baz", "fooBarBaz"},
		{"FooIDbarBaz", "fooIDbarBaz"},
		{"FOOIDbarBaz", "fOOIDbarBaz"},
		{" A B C ", "aBC"},
		{"foo bar baz", "fooBarBaz"},
		{"foo \t \tbar   baz \t", "fooBarBaz"},
		{"foo_bar_baz", "fooBarBaz"},
		{"_foo_bar_baz_", "fooBarBaz"},
		{"foo-bar-baz", "fooBarBaz"},
		{"foo--bar----baz", "fooBarBaz"},
		{"foo__bar____baz", "fooBarBaz"},
		{" Foo Bar Baz ", "fooBarBaz"},
		{" Foo Bar_BazBlah ", "fooBarBazBlah"},
		{" Foo.Bar.Baz", "fooBarBaz"},
		{"#@!Foo@#$@$Bar>>>>>Baz", "fooBarBaz"},
	}

	for _, testCase := range testCases {
		actual := camelCaseLower(testCase.input)
		assert.Equal(t, testCase.expected, actual, "When calling camelCaseLower on '%s', expected '%s', but got '%s'", testCase.input, testCase.expected, actual)
	}
}

func TestLowerFirst(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"     ", "     "},
		{"foo", "foo"},
		{"Foo", "foo"},
		{"FOO", "fOO"},
		{"Здравейте", "здравейте"},
	}

	for _, testCase := range testCases {
		actual := lowerFirst(testCase.input)
		assert.Equal(t, testCase.expected, actual, "When calling lowerFirst on '%s', expected '%s', but got '%s'", testCase.input, testCase.expected, actual)
	}
}

// I cannot believe I have to write my own function and test code for rounding numbers in Go. FML.
func TestRound(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input    float64
		expected int
	}{
		{0, 0},
		{0.0, 0},
		{0.25, 0},
		{0.49, 0},
		{0.4999999999999, 0},
		{0.5, 1},
		{0.50000000000000001, 1},
		{0.75, 1},
		{0.999999999999999, 1},
		{1, 1},
		{1.0, 1},
		{151515151.234234234, 151515151},
	}

	for _, testCase := range testCases {
		actual := round(testCase.input)
		assert.Equal(t, testCase.expected, actual, "When calling round on %f, expected %d, but got %d", testCase.input, testCase.expected, actual)
	}
}

