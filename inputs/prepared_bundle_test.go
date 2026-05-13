package inputs //nolint:testpackage

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPreparedBundle_RenderFilesParityWithRenderFilesFromFS pins the
// load-bearing equivalence: rendering N paths through a PreparedBundle
// must produce byte-identical results to N rendering through the
// existing RenderFilesFromFS entry point. If this ever diverges, the
// WASM bridge's switch from per-call parse to handle-based parse
// changes observable behavior — which is exactly what callers must
// be able to rely on NOT happening.
func TestPreparedBundle_RenderFilesParityWithRenderFilesFromFS(t *testing.T) {
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

	depsIndex := buildDepsIndexFromFS(t, fsys, ".")
	vars := map[string]any{"Region": "us-west-2", "ProjectName": "demo"}
	paths := []string{"README.md", "modules/vpc/main.tf"}

	// Reference: existing RenderFilesFromFS entry point.
	want := RenderFilesFromFS(context.Background(), fsys, ".", paths, vars, depsIndex)

	// New path: same inputs, but through PreparedBundle.
	pb := &PreparedBundle{RootFS: fsys, RootPath: ".", DepsIndex: depsIndex}
	got := pb.RenderFiles(context.Background(), paths, vars)

	require.Len(t, got, len(want))

	for i := range want {
		assert.Equalf(t, want[i].Path, got[i].Path, "result[%d].Path mismatch", i)
		assert.Equalf(t, want[i].Content, got[i].Content, "result[%d].Content mismatch (path=%s)", i, want[i].Path)

		if want[i].Err == nil {
			assert.NoErrorf(t, got[i].Err, "result[%d].Err should be nil to match reference (path=%s)", i, want[i].Path)
		} else {
			assert.Errorf(t, got[i].Err, "result[%d].Err should be non-nil to match reference (path=%s)", i, want[i].Path)
		}
	}
}

// TestPreparedBundle_RenderFileParityWithRenderFileFromFS is the
// single-path counterpart of the above. PreparedBundle.RenderFile is a
// pure passthrough; we still check it because future refactors might
// add caching there and silently change semantics.
func TestPreparedBundle_RenderFileParityWithRenderFileFromFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
		"hello.txt": &fstest.MapFile{Data: []byte(`Hello, {{ .Name }}!`)},
	}

	depsIndex := buildDepsIndexFromFS(t, fsys, ".")
	vars := map[string]any{"Name": "world"}

	want, err := RenderFileFromFS(context.Background(), fsys, ".", "hello.txt", vars, depsIndex)
	require.NoError(t, err)

	pb := &PreparedBundle{RootFS: fsys, RootPath: ".", DepsIndex: depsIndex}
	got, err := pb.RenderFile(context.Background(), "hello.txt", vars)
	require.NoError(t, err)

	assert.Equal(t, want, got)
}

// TestPreparedBundle_RenderFilesPreservesInputOrder pins the same
// ordering invariant RenderFilesFromFS has: results come back in the
// order the caller passed paths, not in fsys walk order or any other
// natural sort.
func TestPreparedBundle_RenderFilesPreservesInputOrder(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(``)},
		"a.txt":           &fstest.MapFile{Data: []byte(`A`)},
		"b.txt":           &fstest.MapFile{Data: []byte(`B`)},
		"c.txt":           &fstest.MapFile{Data: []byte(`C`)},
	}

	pb := &PreparedBundle{RootFS: fsys, RootPath: ".", DepsIndex: map[string][]ResolvedDep{}}

	got := pb.RenderFiles(context.Background(), []string{"c.txt", "a.txt", "b.txt"}, map[string]any{})
	require.Len(t, got, 3)

	assert.Equal(t, "c.txt", got[0].Path)
	assert.Equal(t, "C", got[0].Content)
	assert.Equal(t, "a.txt", got[1].Path)
	assert.Equal(t, "A", got[1].Content)
	assert.Equal(t, "b.txt", got[2].Path)
	assert.Equal(t, "B", got[2].Content)
}

// largeBundleFixture returns a synthetic bundle with the rough shape of
// the gruntwork-landing-zone template (the motivating example in the
// runbooks perf issue): one boilerplate.yml + many text files at the
// root. We don't try to reproduce the dep tree because the per-file
// render work isn't where the amortization win lives — the win lives
// in the bridge's per-call JSON parse + MapFS construction, which
// scales with file count, not dep depth. A flat fixture is enough to
// exercise the same scaling.
func largeBundleFixture(fileCount int) fstest.MapFS {
	fsys := fstest.MapFS{
		"boilerplate.yml": &fstest.MapFile{Data: []byte(`
variables:
  - name: Name
`)},
	}

	for i := 0; i < fileCount; i++ {
		name := fmt.Sprintf("file%03d.txt", i)
		fsys[name] = &fstest.MapFile{Data: []byte(fmt.Sprintf("file %d for {{ .Name }}", i))}
	}

	return fsys
}

// BenchmarkPreparedBundle_RepeatedRenders is the perf-claim acceptance
// test the runbooks team asked for: 10 consecutive renders against the
// same bundle (≥ 50 files) with a small per-call path subset should
// approach N × per-file render cost — i.e., the MapFS construction
// done at PreparedBundle construction time should not be repeated.
//
// The benchmark itself measures only the rendering loop (b.ResetTimer
// is called after PreparedBundle is built). The "fresh-each-call"
// comparison benchmark below explicitly rebuilds the bundle every
// iteration so the delta is visible.
func BenchmarkPreparedBundle_RepeatedRenders(b *testing.B) {
	const (
		fileCount    = 64
		pathsPerCall = 9
	)

	fsys := largeBundleFixture(fileCount)
	depsIndex := map[string][]ResolvedDep{}
	vars := map[string]any{"Name": "world"}

	// Same path subset on every iteration. Real-world callers
	// typically render different per-call subsets, but the
	// per-iteration work is independent of which subset, so a fixed
	// subset keeps the benchmark stable.
	paths := make([]string, 0, pathsPerCall)
	for i := 0; i < pathsPerCall; i++ {
		paths = append(paths, fmt.Sprintf("file%03d.txt", i))
	}

	pb := &PreparedBundle{RootFS: fsys, RootPath: ".", DepsIndex: depsIndex}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results := pb.RenderFiles(context.Background(), paths, vars)
		if len(results) != pathsPerCall {
			b.Fatalf("expected %d results, got %d", pathsPerCall, len(results))
		}
	}
}

// BenchmarkRenderFilesFromFS_RepeatedRenders is the comparison
// benchmark for BenchmarkPreparedBundle_RepeatedRenders. It uses the
// same fixture and same per-call path subset, but runs the existing
// RenderFilesFromFS entry point each time. Because RenderFilesFromFS
// itself doesn't do MapFS construction (the caller already has an
// fs.FS), the Go-level numbers will be similar — the parse savings the
// runbooks issue is about live in the WASM bridge, which Go-level
// benchmarks can't directly observe.
//
// What the two benchmarks DO show, end-to-end, is that PreparedBundle
// doesn't add overhead. If a future change introduces per-call work in
// the PreparedBundle path (locking, hashing, etc.), this side-by-side
// pair will catch it.
func BenchmarkRenderFilesFromFS_RepeatedRenders(b *testing.B) {
	const (
		fileCount    = 64
		pathsPerCall = 9
	)

	fsys := largeBundleFixture(fileCount)
	depsIndex := map[string][]ResolvedDep{}
	vars := map[string]any{"Name": "world"}

	paths := make([]string, 0, pathsPerCall)
	for i := 0; i < pathsPerCall; i++ {
		paths = append(paths, fmt.Sprintf("file%03d.txt", i))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		results := RenderFilesFromFS(context.Background(), fsys, ".", paths, vars, depsIndex)
		if len(results) != pathsPerCall {
			b.Fatalf("expected %d results, got %d", pathsPerCall, len(results))
		}
	}
}
