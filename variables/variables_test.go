package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseStringAsList(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		testName     string
		str          string
		expectedList []string
	}{
		{testName: "empty-list", str: "[]", expectedList: []string{}},
		{testName: "one-item", str: "[a]", expectedList: []string{"a"}},
		{testName: "three-items", str: "[a b c]", expectedList: []string{"a", "b", "c"}},
		{testName: "leading-trailing-whitespace", str: "[ a b c ]", expectedList: []string{"a", "b", "c"}},
		{testName: "json-list-one-item", str: `["a"]`, expectedList: []string{"a"}},
		{testName: "json-list-three-items", str: `["a", "b", "c"]`, expectedList: []string{"a", "b", "c"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			actualList, err := parseStringAsList(testCase.str)
			assert.NoError(t, err, "Got unexpected error for string '%s': %v", testCase.str, err)
			assert.Equal(t, testCase.expectedList, actualList, "For string '%s'", testCase.str)
		})
	}

}

func TestConvertType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		value         interface{}
		expectedValue interface{}
		testName      string
		variableType  BoilerplateType
		expectError   bool
	}{
		// String type tests
		{testName: "string-to-string", value: "hello", variableType: String, expectedValue: "hello", expectError: false},
		{testName: "int-to-string", value: 42, variableType: String, expectedValue: "42", expectError: false},
		{testName: "float64-to-string", value: 3.14, variableType: String, expectedValue: "3.14", expectError: false},
		{testName: "float64-to-string-whole", value: 42.0, variableType: String, expectedValue: "42", expectError: false},
		{testName: "float64-to-string-negative", value: -15.7, variableType: String, expectedValue: "-15.7", expectError: false},
		{testName: "bool-to-string", value: true, variableType: String, expectedValue: "true", expectError: false},
		{testName: "bool-to-string-false", value: false, variableType: String, expectedValue: "false", expectError: false},

		// Int type tests - existing functionality
		{testName: "int-to-int", value: 42, variableType: Int, expectedValue: 42, expectError: false},
		{testName: "string-to-int-valid", value: "123", variableType: Int, expectedValue: 123, expectError: false},
		{testName: "string-to-int-invalid", value: "not-a-number", variableType: Int, expectedValue: nil, expectError: true},

		// Float type tests
		{testName: "float64-to-float", value: 3.14, variableType: Float, expectedValue: 3.14, expectError: false},
		{testName: "string-to-float-valid", value: "3.14", variableType: Float, expectedValue: 3.14, expectError: false},
		{testName: "string-to-float-invalid", value: "not-a-float", variableType: Float, expectedValue: nil, expectError: true},

		// Bool type tests
		{testName: "bool-to-bool-true", value: true, variableType: Bool, expectedValue: true, expectError: false},
		{testName: "bool-to-bool-false", value: false, variableType: Bool, expectedValue: false, expectError: false},
		{testName: "string-to-bool-true", value: "true", variableType: Bool, expectedValue: true, expectError: false},
		{testName: "string-to-bool-false", value: "false", variableType: Bool, expectedValue: false, expectError: false},
		{testName: "string-to-bool-invalid", value: "maybe", variableType: Bool, expectedValue: nil, expectError: true},

		// Nil value test
		{testName: "nil-value", value: nil, variableType: Int, expectedValue: nil, expectError: false},
		{testName: "nil-value-string", value: nil, variableType: String, expectedValue: nil, expectError: false},

		// Invalid type conversions - only test truly invalid conversions
		{testName: "list-to-string-invalid", value: []string{"a", "b"}, variableType: String, expectedValue: nil, expectError: true},
		{testName: "bool-to-int-invalid", value: true, variableType: Int, expectedValue: nil, expectError: true},
		{testName: "list-to-int-invalid", value: []string{"a", "b"}, variableType: Int, expectedValue: nil, expectError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			var variable Variable
			switch testCase.variableType {
			case String:
				variable = NewStringVariable("test-var")
			case Int:
				variable = NewIntVariable("test-var")
			case Float:
				variable = NewFloatVariable("test-var")
			case Bool:
				variable = NewBoolVariable("test-var")
			case List:
				variable = NewListVariable("test-var")
			case Map:
				variable = NewMapVariable("test-var")
			default:
				t.Fatalf("Unsupported variable type in test: %v", testCase.variableType)
			}

			actualValue, err := ConvertType(testCase.value, variable)

			if testCase.expectError {
				assert.Error(t, err, "Expected error for test case: %s", testCase.testName)
			} else {
				assert.NoError(t, err, "Got unexpected error for test case '%s': %v", testCase.testName, err)
				assert.Equal(t, testCase.expectedValue, actualValue, "For test case '%s'", testCase.testName)
			}
		})
	}
}

func TestParseStringAsMap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		expectedMap map[string]string
		testName    string
		str         string
	}{
		{testName: "empty-map", str: "map[]", expectedMap: map[string]string{}},
		{testName: "one-item", str: "map[a:b]", expectedMap: map[string]string{"a": "b"}},
		{testName: "three-items", str: "map[a:b c:d e:f]", expectedMap: map[string]string{"a": "b", "c": "d", "e": "f"}},
		{testName: "multiple-colons", str: "map[a:b:c:d:e]", expectedMap: map[string]string{"a:b:c:d": "e"}},
		{testName: "leading-trailing-whitespace", str: "map[ a:b c:d e:f ]", expectedMap: map[string]string{"a": "b", "c": "d", "e": "f"}},
		{testName: "json-map-empty", str: `{}`, expectedMap: map[string]string{}},
		{testName: "json-map-one-item", str: `{"a": "b"}`, expectedMap: map[string]string{"a": "b"}},
		{testName: "json-map-three-items", str: `{"a": "b", "c": "d", "e": "f"}`, expectedMap: map[string]string{"a": "b", "c": "d", "e": "f"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			actualMap, err := parseStringAsMap(testCase.str)
			assert.NoError(t, err, "Got unexpected error for string '%s': %v", testCase.str, err)
			assert.Equal(t, testCase.expectedMap, actualMap, "For string '%s'", testCase.str)
		})
	}

}
