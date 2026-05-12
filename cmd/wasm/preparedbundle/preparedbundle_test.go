package preparedbundle

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/inputs"
)

// TestBundleStore_StoreReturnsUniqueIDs pins the contract the handlers
// depend on: every Store call hands back a distinct ID even when the
// stored bundle pointer is the same. The JS caller treats the ID as
// opaque, but uniqueness matters because Release on one handle must
// not invalidate another caller's still-live handle.
func TestBundleStore_StoreReturnsUniqueIDs(t *testing.T) {
	t.Parallel()

	store := newBundleStore()
	bundle := &inputs.PreparedBundle{RootPath: "."}

	id1 := store.Store(bundle)
	id2 := store.Store(bundle)

	assert.NotEqual(t, id1, id2, "consecutive Store calls must produce distinct IDs")
	assert.Equal(t, bundle, store.Get(id1))
	assert.Equal(t, bundle, store.Get(id2))
}

// TestBundleStore_ReleaseRemovesHandle pins the post-release behavior:
// Get returns nil so the handler can surface the structural error JS
// callers expect rather than silently rendering against a stale
// bundle.
func TestBundleStore_ReleaseRemovesHandle(t *testing.T) {
	t.Parallel()

	store := newBundleStore()
	id := store.Store(&inputs.PreparedBundle{RootPath: "."})

	require.NotNil(t, store.Get(id), "fresh handle should resolve")
	store.Release(id)
	assert.Nil(t, store.Get(id), "released handle must not resolve")
}

// TestBundleStore_ReleaseUnknownIsNoOp documents the idempotency
// contract: callers don't have to track which handles they've already
// cleaned up; Release on an unknown / already-released handle is a
// no-op.
func TestBundleStore_ReleaseUnknownIsNoOp(t *testing.T) {
	t.Parallel()

	store := newBundleStore()

	// Should not panic, error, or affect state.
	store.Release("pb-does-not-exist")
	store.Release("")
}

// TestBundleStore_DoubleReleaseIsNoOp guards the runbooks-side
// convenience pattern: a try/finally that releases the handle on
// teardown, plus an explicit release on bundle invalidation, must not
// double-fail.
func TestBundleStore_DoubleReleaseIsNoOp(t *testing.T) {
	t.Parallel()

	store := newBundleStore()
	id := store.Store(&inputs.PreparedBundle{RootPath: "."})

	store.Release(id)
	store.Release(id)
	assert.Nil(t, store.Get(id))
}

// TestBundleStore_GetReturnsNilForUnknown is the contract the
// RenderFilesWithHandle handler depends on: any unknown / never-stored
// handle resolves to nil, allowing the handler to surface the
// "unknown or released bundle handle" structural error.
func TestBundleStore_GetReturnsNilForUnknown(t *testing.T) {
	t.Parallel()

	store := newBundleStore()
	assert.Nil(t, store.Get("pb-nope"))
}

// TestBundleStore_ConcurrentAccess guards the mutex. Today's WASM is
// single-threaded so contention is theoretical, but a future
// multi-threaded WASM or a Go-side test exercising the handlers from
// multiple goroutines must not race the underlying map. The race
// detector running this test (under `go test -race`) is the actual
// gate; the assertion is just here to keep the goroutines busy.
func TestBundleStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	const concurrency = 32

	store := newBundleStore()
	bundle := &inputs.PreparedBundle{RootPath: "."}

	var wg sync.WaitGroup

	ids := make([]string, concurrency)
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(i int) {
			defer wg.Done()
			ids[i] = store.Store(bundle)
		}(i)
	}

	wg.Wait()

	// Spot-check that every ID is unique and resolves to the same
	// bundle. Uniqueness is on the atomic counter; map integrity is
	// on the mutex.
	seen := make(map[string]struct{}, concurrency)
	for _, id := range ids {
		assert.NotContains(t, seen, id, "Store handed out a duplicate ID under concurrent load")
		seen[id] = struct{}{}
		assert.Equal(t, bundle, store.Get(id))
	}
}

// TestParseBundle_RoundTripsValidShape exercises the happy path: a
// bundle JSON with a root file, one dep, and a depsIndex entry is
// parsed into a PreparedBundle whose RootFS / RootPath / DepsIndex
// match the input.
func TestParseBundle_RoundTripsValidShape(t *testing.T) {
	t.Parallel()

	bundle := templateBundle{
		RootPath: ".",
		Files: map[string]string{
			"boilerplate.yml": "",
			"hello.txt":       "hi {{ .Name }}",
		},
		Dependencies: map[string][]inputs.ResolvedDep{
			".": {{Name: "child", BundlePath: "_deps/child", OutputFolder: "./child"}},
		},
	}

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	prepared, err := parseBundle(string(data))
	require.NoError(t, err)
	require.NotNil(t, prepared)

	assert.Equal(t, ".", prepared.RootPath)
	assert.NotNil(t, prepared.RootFS, "RootFS must be populated for the renderer to walk")
	assert.Equal(t, bundle.Dependencies, prepared.DepsIndex)
}

// TestParseBundle_DefaultsEmptyRootPathToDot pins the contract that
// the empty rootPath the producer sometimes emits is normalised to
// "." — same convention RenderFileFromFS uses.
func TestParseBundle_DefaultsEmptyRootPathToDot(t *testing.T) {
	t.Parallel()

	prepared, err := parseBundle(`{"rootPath":"","files":{},"dependencies":{}}`)
	require.NoError(t, err)
	assert.Equal(t, ".", prepared.RootPath)
}

// TestParseBundle_RejectsInvalidJSON guards the structural-error path
// the handler returns to JS. A bundle JSON the consumer can't decode
// must surface an error rather than a half-constructed PreparedBundle.
func TestParseBundle_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	prepared, err := parseBundle(`not json at all`)
	require.Error(t, err)
	assert.Nil(t, prepared)
	assert.Contains(t, err.Error(), "failed to parse bundle JSON")
}

// TestParseBundle_RejectsBadFilePath pins the path-validation contract:
// every entry in Files must be a canonical, forward-slash,
// strictly-relative path. An entry with an absolute path is structural
// failure so the JS caller can fall back to cold render rather than
// trust a half-validated bundle.
func TestParseBundle_RejectsBadFilePath(t *testing.T) {
	t.Parallel()

	prepared, err := parseBundle(`{"rootPath":".","files":{"/etc/passwd":""},"dependencies":{}}`)
	require.Error(t, err)
	assert.Nil(t, prepared)
	assert.Contains(t, err.Error(), "absolute paths not allowed")
}

// TestParseBundle_RejectsBadDepBundlePath pins the same rule for
// ResolvedDep.BundlePath. A producer that emits malformed dep
// bundle paths is broken; the bundle is rejected up-front.
func TestParseBundle_RejectsBadDepBundlePath(t *testing.T) {
	t.Parallel()

	bundle := templateBundle{
		RootPath: ".",
		Files:    map[string]string{},
		Dependencies: map[string][]inputs.ResolvedDep{
			".": {{Name: "bad", BundlePath: "../escapes"}},
		},
	}

	data, err := json.Marshal(bundle)
	require.NoError(t, err)

	prepared, err := parseBundle(string(data))
	require.Error(t, err)
	assert.Nil(t, prepared)
	assert.Contains(t, err.Error(), "path escapes bundle root")
}

// TestParseBundle_EndToEndRendersThroughHandle is the contract test
// the runbooks dispatcher will rely on: parse a bundle once, render
// against it many times, and the rendered output is correct on every
// call.
func TestParseBundle_EndToEndRendersThroughHandle(t *testing.T) {
	t.Parallel()

	bundleJSON := `{
  "rootPath": ".",
  "files": {
    "boilerplate.yml": "variables: [{name: Name}]",
    "hello.txt": "Hello, {{ .Name }}!"
  },
  "dependencies": {}
}`

	prepared, err := parseBundle(bundleJSON)
	require.NoError(t, err)

	// Run the render twice. With a handle-based caller, this is the
	// hot path: bundle is parsed once and reused.
	for i := 0; i < 2; i++ {
		results := prepared.RenderFiles(context.Background(), []string{"hello.txt"}, map[string]any{"Name": "world"})
		require.Len(t, results, 1)
		require.NoError(t, results[0].Err)
		assert.Equal(t, "Hello, world!", results[0].Content)
	}
}

// TestClassifyError pins the kind taxonomy the JS caller switches on.
// Same shape as cmd/wasm/renderfiles; duplicating the assertion here
// guards against the two diverging.
func TestClassifyError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"output_not_produced", inputs.ErrOutputNotProduced, "output_not_produced"},
		{"dependency_not_in_bundle", inputs.ErrDependencyNotInBundle, "dependency_not_in_bundle"},
		{"dynamic_filename", inputs.ErrDynamicFilename, "dynamic_filename"},
		{"skip_files_excluded", inputs.ErrSkipFilesExcluded, "skip_files_excluded"},
		{"generic_render_error", errors.New("template execution failed"), "render"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, classifyError(tc.err))
		})
	}
}

// TestLiftInputsToRoot pins the variable-flattening behavior shared
// across the three WASM render packages: top-level "inputs" entries
// are lifted to the root scope so `{{ .Foo }}` works alongside
// `{{ .inputs.Foo }}`, and explicit root-scope keys win over lifted
// ones.
func TestLiftInputsToRoot(t *testing.T) {
	t.Parallel()

	t.Run("nil_returns_empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, liftInputsToRoot(nil))
	})

	t.Run("no_inputs_block_is_passthrough", func(t *testing.T) {
		t.Parallel()
		got := liftInputsToRoot(map[string]any{"A": "1"})
		assert.Equal(t, map[string]any{"A": "1"}, got)
	})

	t.Run("inputs_block_is_lifted", func(t *testing.T) {
		t.Parallel()
		got := liftInputsToRoot(map[string]any{
			"inputs": map[string]any{"Region": "us-east-1"},
		})
		assert.Equal(t, "us-east-1", got["Region"], "inputs.Region must be lifted to .Region")
	})

	t.Run("root_wins_over_lifted", func(t *testing.T) {
		t.Parallel()
		got := liftInputsToRoot(map[string]any{
			"Region": "explicit",
			"inputs": map[string]any{"Region": "lifted"},
		})
		assert.Equal(t, "explicit", got["Region"], "explicit root-scope key must win over inputs-lifted value")
	})
}

// TestValidateBundlePath spot-checks the path rule. Same rule applied
// by cmd/wasm/inputs, cmd/wasm/renderfile, and cmd/wasm/renderfiles —
// keeping the duplicated implementation in sync is the whole point.
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
			err := validateBundlePath(tc.path)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), tc.wantErr)
			}
		})
	}
}
