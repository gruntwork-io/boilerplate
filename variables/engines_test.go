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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockFields := map[string]any{
				"engines": []any{
					map[any]any{
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
