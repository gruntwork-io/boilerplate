package variables

import (
	"testing"
	"reflect"
	"github.com/stretchr/testify/assert"
	"github.com/gruntwork-io/boilerplate/errors"
	"gopkg.in/yaml.v2"
)

const YAML_FILE_ONE_VAR =
`
key: value
`

const YAML_FILE_MULTIPLE_VARS =
`
key1: value1
key2: value2
key3: value3
`

func TestParseVariablesFromVarFileContents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		fileContents  	    string
		expectYamlTypeError bool
		expectedVars        map[string]interface{}
	}{
		{"", false, map[string]interface{}{}},
		{YAML_FILE_ONE_VAR, false, map[string]interface{}{"key": "value"}},
		{YAML_FILE_MULTIPLE_VARS, false, map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{"invalid yaml", true, map[string]interface{}{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromVarFileContents([]byte(testCase.fileContents))
		if testCase.expectYamlTypeError {
			assert.NotNil(t, err)
			unwrapped := errors.Unwrap(err)
			_, isYamlTypeError := unwrapped.(*yaml.TypeError)
			assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(unwrapped))
		} else {
			assert.Nil(t, err, "Got unexpected error: %v", err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		}
	}
}


func TestParseVariablesFromKeyValuePairs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		keyValuePairs []string
		expectedError error
		expectedVars  map[string]interface{}
	}{
		{[]string{}, nil, map[string]interface{}{}},
		{[]string{"key=value"}, nil, map[string]interface{}{"key": "value"}},
		{[]string{"key="}, nil, map[string]interface{}{"key": nil}},
		{[]string{"key1=value1", "key2=value2", "key3=value3"}, nil, map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{[]string{"invalidsyntax"}, InvalidVarSyntax("invalidsyntax"), map[string]interface{}{}},
		{[]string{"="}, VariableNameCannotBeEmpty("="), map[string]interface{}{}},
		{[]string{"=foo"}, VariableNameCannotBeEmpty("=foo"), map[string]interface{}{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromKeyValuePairs(testCase.keyValuePairs)
		if testCase.expectedError == nil {
			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		} else {
			assert.NotNil(t, err)
			assert.True(t, errors.IsError(err, testCase.expectedError), "Expected an error of type '%s' with value '%s' but got an error of type '%s' with value '%s'", reflect.TypeOf(testCase.expectedError), testCase.expectedError.Error(), reflect.TypeOf(err), err.Error())
		}
	}
}
