package variables

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnginesRequiresSupportedTemplateEngine(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		typeStr     string
		expectError bool
	}{
		{
			name:        "gotemplate",
			typeStr:     string(GoTemplate),
			expectError: false,
		},
		{
			name:        "jsonnet",
			typeStr:     string(Jsonnet),
			expectError: false,
		},
		{
			name:        "unsupported",
			typeStr:     "dhall",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		// Capture range variable so it does not change across for loop iterations.
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockFields := map[string]interface{}{
				"engines": []interface{}{
					map[interface{}]interface{}{
						"path":            "foo." + tc.name,
						"template_engine": tc.typeStr,
					},
				},
			}
			_, err := UnmarshalEnginesFromBoilerplateConfigYaml(mockFields)
			if tc.expectError {
				assert.Error(t, err)
				underlyingErr := errors.Unwrap(err)
				var invalidTemplateEngineErr InvalidTemplateEngineErr
				hasType := errors.As(underlyingErr, &invalidTemplateEngineErr)
				assert.True(t, hasType)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
