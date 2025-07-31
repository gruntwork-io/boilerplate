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
		{"empty-list", "[]", []string{}},
		{"one-item", "[a]", []string{"a"}},
		{"three-items", "[a b c]", []string{"a", "b", "c"}},
		{"leading-trailing-whitespace", "[ a b c ]", []string{"a", "b", "c"}},
		{"json-list-one-item", `["a"]`, []string{"a"}},
		{"json-list-three-items", `["a", "b", "c"]`, []string{"a", "b", "c"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			actualList, err := parseStringAsList(testCase.str)
			assert.Nil(t, err, "Got unexpected error for string '%s': %v", testCase.str, err)
			assert.Equal(t, testCase.expectedList, actualList, "For string '%s'", testCase.str)
		})
	}

}

func TestConvertType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		testName      string
		value         interface{}
		variableType  BoilerplateType
		expectedValue interface{}
		expectError   bool
	}{
		// String type tests
		{"string-to-string", "hello", String, "hello", false},
		{"int-to-string", 42, String, "42", false},
		{"float64-to-string", 3.14, String, "3.14", false},
		{"float64-to-string-whole", 42.0, String, "42", false},
		{"float64-to-string-negative", -15.7, String, "-15.7", false},
		{"bool-to-string", true, String, "true", false},
		{"bool-to-string-false", false, String, "false", false},

		// Int type tests - existing functionality
		{"int-to-int", 42, Int, 42, false},
		{"string-to-int-valid", "123", Int, 123, false},
		{"string-to-int-invalid", "not-a-number", Int, nil, true},
		
		// Float type tests
		{"float64-to-float", 3.14, Float, 3.14, false},
		{"string-to-float-valid", "3.14", Float, 3.14, false},
		{"string-to-float-invalid", "not-a-float", Float, nil, true},

		// Bool type tests
		{"bool-to-bool-true", true, Bool, true, false},
		{"bool-to-bool-false", false, Bool, false, false},
		{"string-to-bool-true", "true", Bool, true, false},
		{"string-to-bool-false", "false", Bool, false, false},
		{"string-to-bool-invalid", "maybe", Bool, nil, true},

		// Nil value test
		{"nil-value", nil, Int, nil, false},
		{"nil-value-string", nil, String, nil, false},

		// Invalid type conversions - only test truly invalid conversions
		{"list-to-string-invalid", []string{"a", "b"}, String, nil, true},
		{"bool-to-int-invalid", true, Int, nil, true},
		{"list-to-int-invalid", []string{"a", "b"}, Int, nil, true},
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
				assert.NotNil(t, err, "Expected error for test case: %s", testCase.testName)
			} else {
				assert.Nil(t, err, "Got unexpected error for test case '%s': %v", testCase.testName, err)
				assert.Equal(t, testCase.expectedValue, actualValue, "For test case '%s'", testCase.testName)
			}
		})
	}
}

func TestParseStringAsMap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		testName    string
		str         string
		expectedMap map[string]string
	}{
		{"empty-map", "map[]", map[string]string{}},
		{"one-item", "map[a:b]", map[string]string{"a": "b"}},
		{"three-items", "map[a:b c:d e:f]", map[string]string{"a": "b", "c": "d", "e": "f"}},
		{"multiple-colons", "map[a:b:c:d:e]", map[string]string{"a:b:c:d": "e"}},
		{"leading-trailing-whitespace", "map[ a:b c:d e:f ]", map[string]string{"a": "b", "c": "d", "e": "f"}},
		{"json-map-empty", `{}`, map[string]string{}},
		{"json-map-one-item", `{"a": "b"}`, map[string]string{"a": "b"}},
		{"json-map-three-items", `{"a": "b", "c": "d", "e": "f"}`, map[string]string{"a": "b", "c": "d", "e": "f"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			actualMap, err := parseStringAsMap(testCase.str)
			assert.Nil(t, err, "Got unexpected error for string '%s': %v", testCase.str, err)
			assert.Equal(t, testCase.expectedMap, actualMap, "For string '%s'", testCase.str)
		})
	}

}
