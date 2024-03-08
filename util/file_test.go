package util

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gruntwork-io/boilerplate/errors"
)

func TestIsTextFile(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		file   string
		isText bool
	}{
		{"binary-file.jpg", false},
		{"binary-file.png", false},
		{"binary-file.pdf", false},
		{"binary-file.zip", false},
		{"binary-file", false},
		{"empty-file", false},
		{"text-file.html", true},
		{"text-file.js", true},
		{"text-file.txt", true},
		{"text-file.md", true},
		{"text-file.tf", true},
		{"json-file.json", true},
		{"yaml-file.yaml", true},
	}

	for _, testCase := range testCases {
		actual, err := IsTextFile(fmt.Sprintf("../test-fixtures/util-test/is-text-file/%s", testCase.file))

		assert.Nil(t, err)
		assert.Equal(t, testCase.isText, actual, "Incorrect classification for %s", testCase.file)
	}
}

func TestIsTextFileInvalidPath(t *testing.T) {
	t.Parallel()

	_, err := IsTextFile("invalid-path")
	assert.NotNil(t, err)
	assert.True(t, errors.IsError(err, NoSuchFile("invalid-path")), "Expected NoSuchFile error but got %s", reflect.TypeOf(err))
}
