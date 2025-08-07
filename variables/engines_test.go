package variables_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/variables"
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
			typeStr:     string(variables.GoTemplate),
			expectError: false,
		},
		{
			name:        "jsonnet",
			typeStr:     string(variables.Jsonnet),
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
			_, err := variables.UnmarshalEnginesFromBoilerplateConfigYaml(mockFields)
			if tc.expectError {
				require.Error(t, err)
				underlyingErr := errors.Unwrap(err)
				var invalidTemplateEngineErr variables.InvalidTemplateEngineErr
				hasType := errors.As(underlyingErr, &invalidTemplateEngineErr)
				require.True(t, hasType)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
