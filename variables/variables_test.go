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
