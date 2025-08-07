package util_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	util "github.com/gruntwork-io/boilerplate/util"
)

func TestIsTextFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		file   string
		isText bool
	}{
		{file: "binary-file.jpg", isText: false},
		{file: "binary-file.png", isText: false},
		{file: "binary-file.pdf", isText: false},
		{file: "binary-file.zip", isText: false},
		{file: "binary-file", isText: false},
		{file: "empty-file", isText: false},
		{file: "text-file.html", isText: true},
		{file: "text-file.js", isText: true},
		{file: "text-file.txt", isText: true},
		{file: "text-file.md", isText: true},
		{file: "text-file.tf", isText: true},
		{file: "json-file.json", isText: true},
		{file: "yaml-file.yaml", isText: true},
		{file: "file-go.go", isText: true},
		{file: "file-java.java", isText: true},
		{file: "file-xml.xml", isText: true},
		{file: "file-hcl.hcl", isText: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.file, func(t *testing.T) {
			t.Parallel()
			actual, err := util.IsTextFile("../test-fixtures/util-test/is-text-file/" + testCase.file)

			require.NoError(t, err)
			assert.Equal(t, testCase.isText, actual, "Incorrect classification for %s", testCase.file)
		})
	}
}

func TestIsTextFileInvalidPath(t *testing.T) {
	t.Parallel()

	_, err := util.IsTextFile("invalid-path")
	require.Error(t, err)
	require.ErrorIs(t, err, util.NoSuchFile("invalid-path"), "Expected NoSuchFile error but got %s", reflect.TypeOf(err))
}
