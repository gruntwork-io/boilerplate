package variables //nolint:testpackage

import (
	"errors"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/google/go-cmp/cmp"
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

func TestParserulestring(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		Input string
		Want  string
	}

	testCases := []TestCase{
		{
			Input: "required",
			Want:  "required",
		},
		{
			Input: "REQUIRED",
			Want:  "required",
		},
		{
			Input: "  alphanumeric  ",
			Want:  "alphanumeric",
		},
		{
			Input: "CountryCode2",
			Want:  "countrycode2",
		},
		{
			// Regex patterns preserve case
			Input: "regex(^[A-Z]{2}-\\d{4}$)",
			Want:  "regex(^[A-Z]{2}-\\d{4}$)",
		},
		{
			// Regex patterns with spaces work now (handled via YAML list in production)
			Input: "regex(^[a-z ]+$)",
			Want:  "regex(^[a-z ]+$)",
		},
		{
			// length() is lowercased like other non-regex rules
			Input: "LENGTH(5, 22)",
			Want:  "length(5, 22)",
		},
		{
			Input: "length(1,3)",
			Want:  "length(1,3)",
		},
	}

	for _, tc := range testCases {
		got := normalizeRuleString(tc.Input)

		if !cmp.Equal(got, tc.Want) {
			t.Logf("Got %v for input %s but wanted %v\n", got, tc.Input, tc.Want)
			t.Fail()
		}
	}
}

// normalizeAndConvert is a test helper that normalizes a rule string and converts it
// to a CustomValidationRule in one step.
func normalizeAndConvert(ruleString string) (CustomValidationRule, error) {
	return convertSingleValidationRule(normalizeRuleString(ruleString))
}

// TestConvertSingleValidationRule_Regex tests normalizeRuleString + convertSingleValidationRule
// for regex rules. The production path goes through unmarshalValidationsField, but these tests
// verify the string-parsing pipeline independently.
func TestConvertSingleValidationRule_Regex(t *testing.T) {
	t.Parallel()

	t.Run("lowercase alphanumeric pattern passes", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert("regex(^[a-z0-9]+$)")
		require.NoError(t, err)

		err = rule.Validator.Validate("hello123")
		assert.NoError(t, err)
	})

	t.Run("lowercase alphanumeric pattern rejects uppercase", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert("regex(^[a-z0-9]+$)")
		require.NoError(t, err)

		err = rule.Validator.Validate("Hello!")
		assert.Error(t, err)
	})

	t.Run("case-sensitive pattern passes", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert(`regex(^[A-Z]{2}-\d{4}$)`)
		require.NoError(t, err)

		err = rule.Validator.Validate("AB-1234")
		assert.NoError(t, err)
	})

	t.Run("case-sensitive pattern rejects wrong case", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert(`regex(^[A-Z]{2}-\d{4}$)`)
		require.NoError(t, err)

		err = rule.Validator.Validate("ab-1234")
		assert.Error(t, err)
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeAndConvert("regex(invalid[)")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})

	t.Run("regex with spaces works as single rule", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert("regex(^[a-z ]+$)")
		require.NoError(t, err)

		err = rule.Validator.Validate("hello world")
		require.NoError(t, err)

		err = rule.Validator.Validate("Hello123")
		assert.Error(t, err)
	})
}

func TestConvertSingleValidationRule_Length(t *testing.T) {
	t.Parallel()

	t.Run("length with spaces around args", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert("length(5, 22)")
		require.NoError(t, err)
		assert.Equal(t, "Must be between 5 and 22 characters long", rule.Message)

		err = rule.Validator.Validate("hello")
		assert.NoError(t, err)

		err = rule.Validator.Validate("hi")
		assert.Error(t, err)
	})

	t.Run("length without spaces around args", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert("length(1,3)")
		require.NoError(t, err)
		assert.Equal(t, "Must be between 1 and 3 characters long", rule.Message)

		err = rule.Validator.Validate("ab")
		assert.NoError(t, err)

		err = rule.Validator.Validate("abcd")
		assert.Error(t, err)
	})

	t.Run("length is case-insensitive", func(t *testing.T) {
		t.Parallel()

		rule, err := normalizeAndConvert("LENGTH(10, 30)")
		require.NoError(t, err)
		assert.Equal(t, "Must be between 10 and 30 characters long", rule.Message)
	})

	t.Run("length missing comma returns error", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeAndConvert("length(5)")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected length(min, max)")
	})

	t.Run("length non-numeric min returns error", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeAndConvert("length(abc, 10)")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid min")
	})

	t.Run("length non-numeric max returns error", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeAndConvert("length(5, xyz)")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max")
	})
}

func TestUnmarshalValidationsField_RegexWithSpaces(t *testing.T) {
	t.Parallel()

	t.Run("regex pattern with spaces via YAML list", func(t *testing.T) {
		t.Parallel()

		// Simulate what YAML parsing produces for:
		//   validations:
		//     - required
		//     - "regex(^[a-z ]+$)"
		fields := map[string]any{
			"validations": []interface{}{
				"required",
				"regex(^[a-z ]+$)",
			},
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		require.Len(t, rules, 2)

		assert.Equal(t, "Must not be empty", rules[0].Message)
		assert.Equal(t, "Must match pattern: ^[a-z ]+$", rules[1].Message)

		// Should accept string with spaces
		err = rules[1].Validator.Validate("hello world")
		require.NoError(t, err)

		// Should reject digits
		err = rules[1].Validator.Validate("hello123")
		require.Error(t, err)
	})
}

func TestUnmarshalValidationsField(t *testing.T) {
	t.Parallel()

	t.Run("nil validations returns nil", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		assert.Nil(t, rules)
	})

	t.Run("scalar string returns error", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{
			"validations": "required",
		}

		_, err := unmarshalValidationsField(fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a YAML list, not a string")
	})

	t.Run("regex in list preserves case", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{
			"validations": []interface{}{
				"regex(^[A-Z]{2}-\\d{4}$)",
			},
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		err = rules[0].Validator.Validate("AB-1234")
		require.NoError(t, err)

		err = rules[0].Validator.Validate("ab-1234")
		require.Error(t, err)
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{
			"validations": 42,
		}

		_, err := unmarshalValidationsField(fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a list or string")
	})
}
