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