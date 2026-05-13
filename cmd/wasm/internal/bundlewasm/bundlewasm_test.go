package bundlewasm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/inputs"
)

// TestClassifyError pins the kind taxonomy the JS caller switches on.
// Same shape the runbooks dispatcher dispatches on; consumers (renderfile,
// renderfiles, preparedbundle) all read the result through this helper.
func TestClassifyError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"output_not_produced", inputs.ErrOutputNotProduced, bundlewasm.KindOutputNotProduced},
		{"dependency_not_in_bundle", inputs.ErrDependencyNotInBundle, bundlewasm.KindDependencyNotBundled},
		{"dynamic_filename", inputs.ErrDynamicFilename, bundlewasm.KindDynamicFilename},
		{"skip_files_excluded", inputs.ErrSkipFilesExcluded, bundlewasm.KindSkipFilesExcluded},
		{"generic_render_error", errors.New("template execution failed"), bundlewasm.KindRender},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, bundlewasm.ClassifyError(tc.err))
		})
	}
}

// TestLiftInputsToRoot pins the variable-flattening behavior shared
// across the WASM render packages: top-level "inputs" entries are lifted
// to the root scope so `{{ .Foo }}` works alongside `{{ .inputs.Foo }}`,
// and explicit root-scope keys win over lifted ones.
func TestLiftInputsToRoot(t *testing.T) {
	t.Parallel()

	t.Run("nil_returns_empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, bundlewasm.LiftInputsToRoot(nil))
	})

	t.Run("no_inputs_block_is_passthrough", func(t *testing.T) {
		t.Parallel()

		got := bundlewasm.LiftInputsToRoot(map[string]any{"A": "1"})
		assert.Equal(t, map[string]any{"A": "1"}, got)
	})

	t.Run("inputs_block_is_lifted", func(t *testing.T) {
		t.Parallel()

		got := bundlewasm.LiftInputsToRoot(map[string]any{
			"inputs": map[string]any{"Region": "us-east-1"},
		})
		assert.Equal(t, "us-east-1", got["Region"], "inputs.Region must be lifted to .Region")
	})

	t.Run("root_wins_over_lifted", func(t *testing.T) {
		t.Parallel()

		got := bundlewasm.LiftInputsToRoot(map[string]any{
			"Region": "explicit",
			"inputs": map[string]any{"Region": "lifted"},
		})
		assert.Equal(t, "explicit", got["Region"], "explicit root-scope key must win over inputs-lifted value")
	})
}

func TestValidateBundlePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		path    string
		wantErr string // empty means no error
	}{
		{"valid_relative", "foo/bar.txt", ""},
		{"valid_root", "x.txt", ""},
		{"empty", "", "empty path"},
		{"absolute", "/etc/passwd", "absolute"},
		{"backslash", "foo\\bar", "forward slashes"},
		{"non_canonical", "foo//bar", "non-canonical"},
		{"escapes_root", "../outside", "escapes bundle root"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := bundlewasm.ValidateBundlePath(tc.path)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), tc.wantErr)
			}
		})
	}
}

// TestParseAndLiftVars exercises the two-step parse + lift the render
// handlers all use. Bad JSON surfaces as a structural-shaped error;
// good JSON gets lifted just like LiftInputsToRoot does on its own.
func TestParseAndLiftVars(t *testing.T) {
	t.Parallel()

	t.Run("rejects_invalid_json", func(t *testing.T) {
		t.Parallel()

		_, err := bundlewasm.ParseAndLiftVars(`not json`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse variables JSON")
	})

	t.Run("lifts_inputs_block", func(t *testing.T) {
		t.Parallel()

		got, err := bundlewasm.ParseAndLiftVars(`{"inputs":{"Region":"us-east-1"}}`)
		require.NoError(t, err)
		assert.Equal(t, "us-east-1", got["Region"])
	})
}

// TestParseOutputPaths pins the parse + non-empty contract the bulk
// handlers expose: bad JSON or empty array is structural failure.
func TestParseOutputPaths(t *testing.T) {
	t.Parallel()

	t.Run("rejects_invalid_json", func(t *testing.T) {
		t.Parallel()

		_, err := bundlewasm.ParseOutputPaths(`not json`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse outputPaths JSON")
	})

	t.Run("rejects_empty_array", func(t *testing.T) {
		t.Parallel()

		_, err := bundlewasm.ParseOutputPaths(`[]`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-empty array")
	})

	t.Run("accepts_populated_array", func(t *testing.T) {
		t.Parallel()

		got, err := bundlewasm.ParseOutputPaths(`["a.txt","b.txt"]`)
		require.NoError(t, err)
		assert.Equal(t, []string{"a.txt", "b.txt"}, got)
	})
}
