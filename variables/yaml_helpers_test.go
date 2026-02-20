package variables //nolint:testpackage

import (
	"errors"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const yamlFileOneVar = `
key: value
`

const yamlFileMultipleVars = `
key1: value1
key2: value2
key3: value3
`

func TestParseVariablesFromVarFileContents(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		expectedVars        map[string]any
		fileContents        string
		expectYamlTypeError bool
	}{
		{fileContents: "", expectYamlTypeError: false, expectedVars: map[string]any{}},
		{fileContents: yamlFileOneVar, expectYamlTypeError: false, expectedVars: map[string]any{"key": "value"}},
		{fileContents: yamlFileMultipleVars, expectYamlTypeError: false, expectedVars: map[string]any{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{fileContents: "invalid yaml", expectYamlTypeError: true, expectedVars: map[string]any{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromVarFileContents([]byte(testCase.fileContents))
		if testCase.expectYamlTypeError {
			require.Error(t, err)

			typeError := &yaml.TypeError{}
			isYamlTypeError := errors.As(err, &typeError)
			assert.True(t, isYamlTypeError, "Expected a YAML type error for an invalid yaml file but got %s", reflect.TypeOf(err))
		} else {
			require.NoError(t, err, "Got unexpected error: %v", err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		}
	}
}

func TestParseVariablesFromKeyValuePairs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		expectedError error
		expectedVars  map[string]any
		keyValuePairs []string
	}{
		{keyValuePairs: []string{}, expectedError: nil, expectedVars: map[string]any{}},
		{keyValuePairs: []string{"key=value"}, expectedError: nil, expectedVars: map[string]any{"key": "value"}},
		{keyValuePairs: []string{"key="}, expectedError: nil, expectedVars: map[string]any{"key": nil}},
		{keyValuePairs: []string{"key1=value1", "key2=value2", "key3=value3"}, expectedError: nil, expectedVars: map[string]any{"key1": "value1", "key2": "value2", "key3": "value3"}},
		{keyValuePairs: []string{"key1=left=right"}, expectedError: nil, expectedVars: map[string]any{"key1": "left=right"}},
		{keyValuePairs: []string{"invalidsyntax"}, expectedError: InvalidVarSyntax("invalidsyntax"), expectedVars: map[string]any{}},
		{keyValuePairs: []string{"="}, expectedError: VariableNameCannotBeEmpty("="), expectedVars: map[string]any{}},
		{keyValuePairs: []string{"=foo"}, expectedError: VariableNameCannotBeEmpty("=foo"), expectedVars: map[string]any{}},
	}

	for _, testCase := range testCases {
		actualVars, err := parseVariablesFromKeyValuePairs(testCase.keyValuePairs)
		if testCase.expectedError == nil {
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedVars, actualVars)
		} else {
			require.Error(t, err)
			assert.ErrorIs(t, err, testCase.expectedError, "Expected an error of type '%s' with value '%s' but got an error of type '%s' with value '%s'", reflect.TypeOf(testCase.expectedError), testCase.expectedError.Error(), reflect.TypeOf(err), err.Error())
		}
	}
}

func TestConvert(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input        any
		expectedType any
	}{
		{
			input:        "",
			expectedType: "string",
		},
		{
			input: map[any]any{
				"key1": "value1",
				"key2": "value2",
			},
			expectedType: map[string]any{},
		},
		{
			input: map[string]any{
				"key1": "value1",
			},
			expectedType: map[string]any{},
		},
		{
			input: []any{
				map[any]any{
					"key1": "value1",
				},
				"",
			},
			expectedType: []any{},
		},
		{
			input: map[string]any{
				"key3": 42,
				"key1": map[string]any{
					"key2": "value2",
				},
			},
			expectedType: map[string]any{},
		},
	}

	for _, testCase := range testCases {
		actual, err := ConvertYAMLToStringMap(testCase.input)
		require.NoError(t, err)
		assert.IsType(t, testCase.expectedType, actual)
	}
}

func TestConvertNested(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input any
		name  string
	}{
		{
			name: "map nested in map",
			input: map[string]any{
				"key1": map[any]any{
					"nested": "value",
				},
			},
		},
		{
			name: "map nested in list",
			input: []any{
				map[any]any{
					"nested": "value",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := ConvertYAMLToStringMap(testCase.input)
			require.NoError(t, err)

			// Check that conversion actually happened - MUST work, not optional
			switch v := result.(type) {
			case map[string]any:
				for _, value := range v {
					nestedMap, ok := value.(map[string]any)
					assert.True(t, ok, "Expected nested map[string]any, got %T", value)
					assert.Equal(t, "value", nestedMap["nested"])
				}
			case []any:
				for _, item := range v {
					nestedMap, ok := item.(map[string]any)
					assert.True(t, ok, "Expected nested map[string]any, got %T", item)
					assert.Equal(t, "value", nestedMap["nested"])
				}
			}
		})
	}
}
