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
		{
			// Regex patterns without spaces work through the string path.
			Input: "[required regex(^[A-Z]{2}-\\d{4}$)]",
			Want:  []string{"required", "regex(^[A-Z]{2}-\\d{4}$)"},
		},
	}

	for _, tc := range testCases {
		got, err := parseRuleString(tc.Input)
		require.NoError(t, err)
		if !cmp.Equal(got, tc.Want) {
			t.Logf("Got %v for input %s but wanted %v\n", got, tc.Input, tc.Want)
			t.Fail()
		}
	}
}

func TestParserulestring_RegexWithSpacesErrors(t *testing.T) {
	t.Parallel()

	_, err := parseRuleString("[required regex(^[a-z ]+$)]")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regex pattern appears to contain a space")
}

// TestConvertValidationStringtoRules_Regex tests the public ConvertValidationStringtoRules API,
// which uses parseRuleString + convertSingleValidationRule. This function has no production
// callers (the production path goes through unmarshalValidationsField), but these tests verify
// the string-parsing pipeline independently.
func TestConvertValidationStringtoRules_Regex(t *testing.T) {
	t.Parallel()

	t.Run("lowercase alphanumeric pattern passes", func(t *testing.T) {
		t.Parallel()
		rules, err := ConvertValidationStringtoRules("[regex(^[a-z0-9]+$)]")
		require.NoError(t, err)
		require.Len(t, rules, 1)

		err = rules[0].Validator.Validate("hello123")
		assert.NoError(t, err)
	})

	t.Run("lowercase alphanumeric pattern rejects uppercase", func(t *testing.T) {
		t.Parallel()
		rules, err := ConvertValidationStringtoRules("[regex(^[a-z0-9]+$)]")
		require.NoError(t, err)
		require.Len(t, rules, 1)

		err = rules[0].Validator.Validate("Hello!")
		assert.Error(t, err)
	})

	t.Run("case-sensitive pattern passes", func(t *testing.T) {
		t.Parallel()
		rules, err := ConvertValidationStringtoRules(`[regex(^[A-Z]{2}-\d{4}$)]`)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		err = rules[0].Validator.Validate("AB-1234")
		assert.NoError(t, err)
	})

	t.Run("case-sensitive pattern rejects wrong case", func(t *testing.T) {
		t.Parallel()
		rules, err := ConvertValidationStringtoRules(`[regex(^[A-Z]{2}-\d{4}$)]`)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		err = rules[0].Validator.Validate("ab-1234")
		assert.Error(t, err)
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		t.Parallel()
		_, err := ConvertValidationStringtoRules("[regex(invalid[)]")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})

	t.Run("regex works alongside other validations", func(t *testing.T) {
		t.Parallel()
		rules, err := ConvertValidationStringtoRules("[required regex(^[a-z]+$)]")
		require.NoError(t, err)
		require.Len(t, rules, 2)

		assert.Equal(t, "Must not be empty", rules[0].Message)
		assert.Equal(t, "Must match pattern: ^[a-z]+$", rules[1].Message)

		// required should reject empty string
		err = rules[0].Validator.Validate("")
		assert.Error(t, err)

		// regex should accept valid input
		err = rules[1].Validator.Validate("hello")
		assert.NoError(t, err)

		// regex should reject invalid input
		err = rules[1].Validator.Validate("Hello123")
		assert.Error(t, err)
	})

	t.Run("regex with spaces in string format returns error", func(t *testing.T) {
		t.Parallel()
		_, err := ConvertValidationStringtoRules("[required regex(^[a-z ]+$)]")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "regex pattern appears to contain a space")
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
		assert.NoError(t, err)

		// Should reject digits
		err = rules[1].Validator.Validate("hello123")
		assert.Error(t, err)
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

	t.Run("required as scalar", func(t *testing.T) {
		t.Parallel()
		fields := map[string]any{
			"validations": "required",
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, "Must not be empty", rules[0].Message)

		err = rules[0].Validator.Validate("")
		assert.Error(t, err)

		err = rules[0].Validator.Validate("something")
		assert.NoError(t, err)
	})

	t.Run("email as scalar", func(t *testing.T) {
		t.Parallel()
		fields := map[string]any{
			"validations": "email",
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, "Must be a valid email address", rules[0].Message)

		err = rules[0].Validator.Validate("user@example.com")
		assert.NoError(t, err)

		err = rules[0].Validator.Validate("not-an-email")
		assert.Error(t, err)
	})

	t.Run("regex as scalar preserves case", func(t *testing.T) {
		t.Parallel()
		fields := map[string]any{
			"validations": "regex(^[A-Z]+$)",
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		require.Len(t, rules, 1)

		err = rules[0].Validator.Validate("HELLO")
		assert.NoError(t, err)

		err = rules[0].Validator.Validate("hello")
		assert.Error(t, err)
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
		assert.NoError(t, err)

		err = rules[0].Validator.Validate("ab-1234")
		assert.Error(t, err)
	})

	t.Run("scalar string is case-insensitive for non-regex rules", func(t *testing.T) {
		t.Parallel()
		fields := map[string]any{
			"validations": "Required",
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		require.Len(t, rules, 1)
		assert.Equal(t, "Must not be empty", rules[0].Message)
	})

	t.Run("unrecognized scalar returns no rules", func(t *testing.T) {
		t.Parallel()
		fields := map[string]any{
			"validations": "notarealrule",
		}

		rules, err := unmarshalValidationsField(fields)
		require.NoError(t, err)
		assert.Nil(t, rules)
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
