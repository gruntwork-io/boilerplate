package inputs //nolint:testpackage

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderFiles is the bulk counterpart to renderFile in
// render_file_test.go: it builds a deps index from the in-memory fixture
// the same way and invokes RenderFilesFromFS so test cases stay terse.
func renderFiles(t *testing.T, fsys fstest.MapFS, paths []string, vars map[string]any) []RenderFileResult {
	t.Helper()

	idx := buildDepsIndexFromFS(t, fsys, ".")

	return RenderFilesFromFS(context.Background(), fsys, ".", paths, vars, idx)
}

// TestRenderFilesFromFS_SmokeParityMatchesSingleRender pins the load-bearing
// equivalence: bulk-rendering N paths must produce byte-identical content
// to N separate RenderFileFromFS calls against the same bundle and vars.
// This mirrors the spec's "Smoke parity" test case.
func TestRenderFilesFromFS_SmokeParityMatchesSingleRender(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
  - name: ProjectName
dependencies:
  - name: vpc
    template-url: ./modules/vpc
    output-folder: ./modules/vpc
    variables:
      - name: AwsRegion
        default: "{{ .Region }}"
`)},
		"README.md": &fstest.MapFile{Data: []byte(`# {{ .ProjectName }}
Deployed in {{ .Region }}.
`)},

		"modules/vpc/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: AwsRegion
`)},
		"modules/vpc/main.tf": &fstest.MapFile{Data: []byte(`provider "aws" { region = "{{ .AwsRegion }}" }
`)},
	}

	vars := map[string]any{"Region": "us-west-2", "ProjectName": "demo"}

	wantREADME, err := renderFile(t, fsys, "README.md", vars)
	require.NoError(t, err)
	wantTF, err := renderFile(t, fsys, "modules/vpc/main.tf", vars)
	require.NoError(t, err)

	got := renderFiles(t, fsys, []string{"README.md", "modules/vpc/main.tf"}, vars)
	require.Len(t, got, 2)

	require.NoError(t, got[0].Err)
	assert.Equal(t, "README.md", got[0].Path)
	assert.Equal(t, wantREADME, got[0].Content)

	require.NoError(t, got[1].Err)
	assert.Equal(t, "modules/vpc/main.tf", got[1].Path)
	assert.Equal(t, wantTF, got[1].Content)
}

// TestRenderFilesFromFS_PartialFailureDoesNotAbortSiblings is the
// load-bearing UX guarantee from the spec: a broken template in one file
// must not blank the preview pane for the others. The broken file gets a
// non-nil Err; siblings continue and succeed.
func TestRenderFilesFromFS_PartialFailureDoesNotAbortSiblings(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
		// ok.txt renders cleanly.
		"ok.txt": &fstest.MapFile{Data: []byte(`hello, {{ .Name }}`)},
		// broken.txt references a variable that doesn't exist in scope,
		// which the engine surfaces under missingkey=error.
		"broken.txt": &fstest.MapFile{Data: []byte(`{{ .Missing }}`)},
		// also-ok.txt comes after the broken one; it must still render.
		"also-ok.txt": &fstest.MapFile{Data: []byte(`bye, {{ .Name }}`)},
	}

	got := renderFiles(t, fsys, []string{"ok.txt", "broken.txt", "also-ok.txt"}, map[string]any{
		"Name": "alice",
	})
	require.Len(t, got, 3)

	require.NoError(t, got[0].Err)
	assert.Equal(t, "hello, alice", got[0].Content)

	require.Error(t, got[1].Err, "broken template must produce an error")
	assert.Equal(t, "broken.txt", got[1].Path)
	// The broken-template error is wrapped, not one of the routing
	// sentinels — the WASM boundary classifies it as "render".
	require.NotErrorIs(t, got[1].Err, ErrOutputNotProduced)
	require.NotErrorIs(t, got[1].Err, ErrDynamicFilename)
	require.NotErrorIs(t, got[1].Err, ErrSkipFilesExcluded)

	require.NoError(t, got[2].Err)
	assert.Equal(t, "bye, alice", got[2].Content)
}

// TestRenderFilesFromFS_PreservesInputOrder verifies the order invariant
// when input ordering differs from any natural iteration order the fsys
// might prefer.
func TestRenderFilesFromFS_PreservesInputOrder(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"a.txt":           &fstest.MapFile{Data: []byte(`A`)},
		"b.txt":           &fstest.MapFile{Data: []byte(`B`)},
		"c.txt":           &fstest.MapFile{Data: []byte(`C`)},
	}

	// Intentionally not alphabetical.
	got := renderFiles(t, fsys, []string{"c.txt", "a.txt", "b.txt"}, map[string]any{})
	require.Len(t, got, 3)

	assert.Equal(t, "c.txt", got[0].Path)
	assert.Equal(t, "C", got[0].Content)

	assert.Equal(t, "a.txt", got[1].Path)
	assert.Equal(t, "A", got[1].Content)

	assert.Equal(t, "b.txt", got[2].Path)
	assert.Equal(t, "B", got[2].Content)
}

// TestRenderFilesFromFS_UnknownPathInlineNotAbort verifies an unknown
// output path returns ErrOutputNotProduced inline for that entry only,
// while siblings render successfully. Same routing signal as the
// single-path API.
func TestRenderFilesFromFS_UnknownPathInlineNotAbort(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"real.txt":        &fstest.MapFile{Data: []byte(`real`)},
	}

	got := renderFiles(t, fsys, []string{"real.txt", "ghost.txt"}, map[string]any{})
	require.Len(t, got, 2)

	require.NoError(t, got[0].Err)
	assert.Equal(t, "real", got[0].Content)

	require.Error(t, got[1].Err)
	assert.ErrorIs(t, got[1].Err, ErrOutputNotProduced,
		"unknown sibling must produce ErrOutputNotProduced inline; got: %v", got[1].Err)
}

// TestRenderFilesFromFS_SkipFilesExcludedInline verifies that requesting a
// file the dep's skip_files rule excludes returns ErrSkipFilesExcluded
// inline rather than aborting the batch.
func TestRenderFilesFromFS_SkipFilesExcludedInline(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Region
skip_files:
  - path: secrets.txt
`)},
		"main.tf":     &fstest.MapFile{Data: []byte(`region = "{{ .Region }}"`)},
		"secrets.txt": &fstest.MapFile{Data: []byte(`secret={{ .Region }}`)},
	}

	got := renderFiles(t, fsys, []string{"main.tf", "secrets.txt"}, map[string]any{
		"Region": "us-east-1",
	})
	require.Len(t, got, 2)

	require.NoError(t, got[0].Err)
	assert.Equal(t, `region = "us-east-1"`, got[0].Content)

	require.Error(t, got[1].Err)
	assert.ErrorIs(t, got[1].Err, ErrSkipFilesExcluded,
		"excluded file must produce ErrSkipFilesExcluded inline; got: %v", got[1].Err)
}

// TestRenderFilesFromFS_EmptyPathsReturnsEmptySlice pins the no-paths
// behavior at the Go API: zero input paths → zero results, no panic. The
// non-empty requirement in the spec is enforced at the WASM boundary
// (where empty arrays are structural Errors), not in this lower-level API.
func TestRenderFilesFromFS_EmptyPathsReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"any.txt":         &fstest.MapFile{Data: []byte(`x`)},
	}

	got := renderFiles(t, fsys, nil, map[string]any{})
	assert.Empty(t, got)
}

// TestRenderFilesFromFS_ForEachBulkRender is the bulk-path reproduction
// of the gruntwork-landing-zone bug: 20 of 63 outputs were failing under
// boilerplateRenderFiles because the warm path never seeded __each__
// before evaluating dep-variable defaults. This test asks the bulk API
// for every for_each-derived output at once and asserts each entry has
// no Err and the correct content — the failure mode the runbooks side
// had to work around by routing kind:"render" to cold.
func TestRenderFilesFromFS_ForEachBulkRender(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
dependencies:
  - name: envs
    template-url: ./envs
    output-folder: "pipelines-config/{{ .__each__ }}"
    for_each:
      - dev
      - staging
      - prod
    variables:
      - name: FormattedAccountName
        default: "acct-{{ .__each__ }}"
`)},
		"envs/boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: FormattedAccountName
`)},
		"envs/account.hcl": &fstest.MapFile{Data: []byte(`account = "{{ .FormattedAccountName }}"`)},
	}

	paths := []string{
		"pipelines-config/dev/account.hcl",
		"pipelines-config/staging/account.hcl",
		"pipelines-config/prod/account.hcl",
	}

	got := renderFiles(t, fsys, paths, map[string]any{})
	require.Len(t, got, 3)

	require.NoError(t, got[0].Err)
	assert.Equal(t, `account = "acct-dev"`, got[0].Content)

	require.NoError(t, got[1].Err)
	assert.Equal(t, `account = "acct-staging"`, got[1].Content)

	require.NoError(t, got[2].Err)
	assert.Equal(t, `account = "acct-prod"`, got[2].Content)
}
