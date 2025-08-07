package variables

import (
	"errors"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

const YAML_FILE_ONE_VAR = `
key: value
`

const YAML_FILE_MULTIPLE_VARS = `
key1: value1
key2: value2
key3: value3
`

func TestParseVariablesFromVarFileContents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		expectedVars        map[string]interface{}
		fileContents        string
		expectYamlTypeError bool
	}{
		{fileContents: "", expectYamlTypeError: false, expectedVars: map[string]interface{}{}},
		{fileContents: YAML_FILE_ONE_VAR, expectYamlTypeError: false, expectedVars: map[string]interface{}{"key": "value"}},
		{fileContents: YAML_FILE_MULTIPLE_VARS, expectYamlTypeError: false, expectedVars: map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{fileContents: "invalid yaml", expectYamlTypeError: true, expectedVars: map[string]interface{}{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromVarFileContents([]byte(testCase.fileContents))
		if testCase.expectYamlTypeError {
			assert.Error(t, err)
			typeError := &yaml.TypeError{}
			isYamlTypeError := errors.As(err, &typeError)
			assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(err))
		} else {
			assert.NoError(t, err, "Got unexpected error: %v", err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		}
	}
}

func TestParseVariablesFromKeyValuePairs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		expectedError error
		expectedVars  map[string]interface{}
		keyValuePairs []string
	}{
		{keyValuePairs: []string{}, expectedError: nil, expectedVars: map[string]interface{}{}},
		{keyValuePairs: []string{"key=value"}, expectedError: nil, expectedVars: map[string]interface{}{"key": "value"}},
		{keyValuePairs: []string{"key="}, expectedError: nil, expectedVars: map[string]interface{}{"key": nil}},
		{keyValuePairs: []string{"key1=value1", "key2=value2", "key3=value3"}, expectedError: nil, expectedVars: map[string]interface{}{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{keyValuePairs: []string{"key1=left=right"}, expectedError: nil, expectedVars: map[string]interface{}{"key1": "left=right"}},
		{keyValuePairs: []string{"invalidsyntax"}, expectedError: InvalidVarSyntax("invalidsyntax"), expectedVars: map[string]interface{}{}},
		{keyValuePairs: []string{"="}, expectedError: VariableNameCannotBeEmpty("="), expectedVars: map[string]interface{}{}},
		{keyValuePairs: []string{"=foo"}, expectedError: VariableNameCannotBeEmpty("=foo"), expectedVars: map[string]interface{}{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromKeyValuePairs(testCase.keyValuePairs)
		if testCase.expectedError == nil {
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		} else {
			assert.Error(t, err)
			assert.ErrorIs(t, err, testCase.expectedError, "Expected an error of type '%s' with value '%s' but got an error of type '%s' with value '%s'", reflect.TypeOf(testCase.expectedError), testCase.expectedError.Error(), reflect.TypeOf(err), err.Error())
		}
	}
}

func TestConvert(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input        interface{}
		expectedType interface{}
	}{
		{
			input:        "",
			expectedType: "string",
		},
		{
			input: map[interface{}]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			expectedType: map[string]interface{}{},
		},
		{
			input: map[string]interface{}{
				"key1": "value1",
			},
			expectedType: map[string]interface{}{},
		},
		{
			input: []interface{}{
				map[interface{}]interface{}{
					"key1": "value1",
				},
				"",
			},
			expectedType: []interface{}{},
		},
		{
			input: map[string]interface{}{
				"key3": 42,
				"key1": map[string]interface{}{
					"key2": "value2",
				},
			},
			expectedType: map[string]interface{}{},
		},
	}

	for _, testCase := range testCases {
		actual, err := ConvertYAMLToStringMap(testCase.input)
		assert.NoError(t, err)
		assert.IsType(t, testCase.expectedType, actual)
	}
}

func TestConvertNested(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input interface{}
		name  string
	}{
		{
			name: "map nested in map",
			input: map[string]interface{}{
				"key1": map[interface{}]interface{}{
					"nested": "value",
				},
			},
		},
		{
			name: "map nested in list",
			input: []interface{}{
				map[interface{}]interface{}{
					"nested": "value",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := ConvertYAMLToStringMap(testCase.input)
			assert.NoError(t, err)

			// Check that conversion actually happened - MUST work, not optional
			switch v := result.(type) {
			case map[string]interface{}:
				for _, value := range v {
					nestedMap, ok := value.(map[string]interface{})
					assert.True(t, ok, "Expected nested map[string]interface{}, got %T", value)
					assert.Equal(t, "value", nestedMap["nested"])
				}
			case []interface{}:
				for _, item := range v {
					nestedMap, ok := item.(map[string]interface{})
					assert.True(t, ok, "Expected nested map[string]interface{}, got %T", item)
					assert.Equal(t, "value", nestedMap["nested"])
				}
			}
		})
	}
}

func TestParserulestring(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		Input string
		Want  []string
	}

	testCases := []TestCase{
		{
			Input: "[required length-5-22 alphanumeric]",
			Want:  []string{"required", "length-5-22", "alphanumeric"},
		},
		{
			Input: "[required]",
			Want:  []string{"required"},
		},
		{
			Input: "[alphanumeric length-10-30]",
			Want:  []string{"alphanumeric", "length-10-30"},
		},
		{
			Input: "[length-1-3 required url email alpha digit alphanumeric CountryCode2]",
			Want:  []string{"length-1-3", "required", "url", "email", "alpha", "digit", "alphanumeric", "countrycode2"},
		},
		{
			Input: "[LENGTH-1-3 REQUIRED URL EMAIL ALPHA DIGIT ALPHANUMERIC COUNTRYCODE2]",
			Want:  []string{"length-1-3", "required", "url", "email", "alpha", "digit", "alphanumeric", "countrycode2"},
		},
	}

	for _, tc := range testCases {
		got := parseRuleString(tc.Input)
		if !cmp.Equal(got, tc.Want) {
			t.Logf("Got %v for input %s but wanted %v\n", got, tc.Input, tc.Want)
			t.Fail()
		}
	}
}
