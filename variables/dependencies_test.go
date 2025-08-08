package variables_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gruntwork-io/boilerplate/variables"
)

func TestSplitIntoDependencyNameAndVariableName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		variableName                 string
		expectedDependencyName       string
		expectedOriginalVariableName string
	}{
		{"", "", ""},
		{"foo", "", "foo"},
		{"foo-bar baz_blah", "", "foo-bar baz_blah"},
		{"foo.bar", "foo", "bar"},
		{"foo.bar.baz", "foo", "bar.baz"},
	}

	for _, testCase := range testCases {
		actualDependencyName, actualOriginalVariableName := variables.SplitIntoDependencyNameAndVariableName(testCase.variableName)
		assert.Equal(t, testCase.expectedDependencyName, actualDependencyName, "Variable name: %s", testCase.variableName)
		assert.Equal(t, testCase.expectedOriginalVariableName, actualOriginalVariableName, "Variable name: %s", testCase.variableName)
	}
}
