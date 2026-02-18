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
			// Only whitespace is trimmed; casing is preserved
			Input: "  alphanumeric  ",
			Want:  "alphanumeric",
		},
		{
			// Casing is preserved as-is
			Input: "REQUIRED",
			Want:  "REQUIRED",
		},
		{
			Input: `regex("^[A-Z]{2}-\d{4}$")`,
			Want:  `regex("^[A-Z]{2}-\d{4}$")`,
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

	// Backtick-quoted rule inputs use regular Go string literals (not raw literals)
	// because the rule itself contains backtick characters. The \\d sequences become
	// \d at runtime.
	validCases := []struct {
		name          string
		ruleInput     string
		validValues   []string
		invalidValues []string
	}{
		{
			name:          "lowercase alphanumeric double-quoted pattern",
			ruleInput:     `regex("^[a-z0-9]+$")`,
			validValues:   []string{"hello123"},
			invalidValues: []string{"Hello!"},
		},
		{
			name:          "case-sensitive backtick-quoted pattern",
			ruleInput:     "regex(`^[A-Z]{2}-\\d{4}$`)",
			validValues:   []string{"AB-1234"},
			invalidValues: []string{"ab-1234"},
		},
		{
			name:          "double-quoted pattern with backslash shorthand",
			ruleInput:     `regex("^[A-Z]{2}-\d{4}$")`,
			validValues:   []string{"AB-1234"},
			invalidValues: []string{"AB-XXXX"},
		},
		{
			name:          "pattern with spaces",
			ruleInput:     `regex("^[a-z ]+$")`,
			validValues:   []string{"hello world"},
			invalidValues: []string{"Hello123"},
		},
		{
			name:          "double-quoted pattern with escaped quotes",
			ruleInput:     "regex(`They said: \"Hello world!\"`)",
			validValues:   []string{`They said: "Hello world!"`},
			invalidValues: []string{`They said something else`},
		},
		{
			name:          "double-quoted pattern with escaped quotes",
			ruleInput:     `regex("They said: \"Hello world!\"")`,
			validValues:   []string{`They said: "Hello world!"`},
			invalidValues: []string{`They said something else`},
		},
		{
			name:          "backtick-quoted pattern",
			ruleInput:     "regex(`^[a-z0-9-]+$`)",
			validValues:   []string{"hello-world-123"},
			invalidValues: []string{"Hello World!"},
		},
		{
			name:          "backtick-quoted pattern with literal quotes",
			ruleInput:     "regex(`They said: \"Hello world!\"`)",
			validValues:   []string{`They said: "Hello world!"`},
			invalidValues: []string{`no match here`},
		},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rule, err := normalizeAndConvert(tc.ruleInput)
			require.NoError(t, err)

			for _, val := range tc.validValues {
				require.NoError(t, rule.Validator.Validate(val), "expected %q to match", val)
			}

			for _, val := range tc.invalidValues {
				assert.Error(t, rule.Validator.Validate(val), "expected %q to not match", val)
			}
		})
	}

	errorCases := []struct {
		name        string
		ruleInput   string
		errContains string
	}{
		{
			name:        "invalid regex returns error",
			ruleInput:   `regex("invalid[")`,
			errContains: "invalid regex pattern",
		},
		{
			name:        "missing quotes returns error",
			ruleInput:   `regex(^[A-Z]{2}-\d{4}$)`,
			errContains: "pattern must be a quoted string",
		},
		{
			name:        "unescaped inner quotes returns error",
			ruleInput:   `regex("They said: "Hello world!"")`,
			errContains: "unescaped",
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeAndConvert(tc.ruleInput)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestUnquoteRegexPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		want        string
		errContains string
		wantErr     bool
	}{
		{name: "double-quoted simple", input: `"^[a-z]+$"`, want: `^[a-z]+$`},
		{name: "backtick-quoted simple", input: "`^[a-z]+$`", want: `^[a-z]+$`},
		{name: `\d passes through`, input: `"^\d{3}$"`, want: `^\d{3}$`},
		{name: `\w passes through`, input: `"^\w+$"`, want: `^\w+$`},
		{name: `\s passes through`, input: `"\s+"`, want: `\s+`},
		{name: `\" unescapes to quote`, input: `"say \"hi\""`, want: `say "hi"`},
		{name: `\\ unescapes to backslash`, input: `"a\\\\b"`, want: `a\\b`},
		{name: "backtick preserves everything", input: "`\\d \\w \\\\ \\\"`", want: `\d \w \\ \"`},
		{name: "empty double-quoted", input: `""`, want: ""},
		{name: "empty backtick", input: "``", want: ""},
		{name: "empty string", input: ``, wantErr: true, errContains: "pattern must be a quoted string"},
		{name: "mismatched quotes", input: "`foo\"", wantErr: true, errContains: "pattern must be a quoted string"},
		{name: "unquoted string", input: `hello`, wantErr: true, errContains: "pattern must be a quoted string"},
		{name: "unescaped inner quote", input: `"foo"bar"`, wantErr: true, errContains: "unescaped"},
		{name: "unescaped inner quotes in sentence", input: `"They said: "Hello world!""`, wantErr: true, errContains: "unescaped"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := unquoteRegexPattern(tc.input)
			if tc.wantErr {
				require.Error(t, err)

				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConvertSingleValidationRule_Length(t *testing.T) {
	t.Parallel()

	validCases := []struct {
		name            string
		ruleInput       string
		expectedMessage string
		validValues     []string
		invalidValues   []string
	}{
		{
			name:            "with spaces around args",
			ruleInput:       "length(5, 22)",
			expectedMessage: "Must be between 5 and 22 characters long",
			validValues:     []string{"hello"},
			invalidValues:   []string{"hi"},
		},
		{
			name:            "without spaces around args",
			ruleInput:       "length(1,3)",
			expectedMessage: "Must be between 1 and 3 characters long",
			validValues:     []string{"ab"},
			invalidValues:   []string{"abcd"},
		},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rule, err := normalizeAndConvert(tc.ruleInput)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedMessage, rule.Message)

			for _, val := range tc.validValues {
				require.NoError(t, rule.Validator.Validate(val), "expected %q to pass", val)
			}

			for _, val := range tc.invalidValues {
				assert.Error(t, rule.Validator.Validate(val), "expected %q to fail", val)
			}
		})
	}

	errorCases := []struct {
		name        string
		ruleInput   string
		errContains string
	}{
		{
			name:        "wrong case returns error",
			ruleInput:   "LENGTH(10, 30)",
			errContains: "unrecognized validation rule",
		},
		{
			name:        "missing comma returns error",
			ruleInput:   "length(5)",
			errContains: "expected length(min, max)",
		},
		{
			name:        "non-numeric min returns error",
			ruleInput:   "length(abc, 10)",
			errContains: "invalid min",
		},
		{
			name:        "non-numeric max returns error",
			ruleInput:   "length(5, xyz)",
			errContains: "invalid max",
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeAndConvert(tc.ruleInput)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestUnmarshalValidationsField_RegexWithSpaces(t *testing.T) {
	t.Parallel()

	t.Run("regex pattern with spaces via YAML list", func(t *testing.T) {
		t.Parallel()

		// Simulate what YAML parsing produces for:
		//   validations:
		//     - required
		//     - 'regex("^[a-z ]+$")'
		fields := map[string]any{
			"validations": []any{
				"required",
				`regex("^[a-z ]+$")`,
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
		assert.Empty(t, rules)
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

		// Use backtick quoting since the pattern contains \d
		fields := map[string]any{
			"validations": []any{
				// Alternatively, we could use double-quoted strings:
				// `regex("^[A-Z]{2}-\d{4}$")``,
				"regex(`^[A-Z]{2}-\\d{4}$`)",
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

	t.Run("non-string item in list returns error", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{
			"validations": []any{
				"required",
				42,
			},
		}

		_, err := unmarshalValidationsField(fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string, got int")
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		t.Parallel()

		fields := map[string]any{
			"validations": 42,
		}

		_, err := unmarshalValidationsField(fields)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a list of strings")
	})
}
