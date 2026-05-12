package inputs

import (
	"context"
	"io/fs"
)

// PreparedBundle is a pre-parsed snapshot of everything RenderFileFromFS
// needs: the bundle's in-memory filesystem, its root path, and the
// dependency index the CLI recorded at bundle time. It carries no
// per-render state and is safe to reuse across many render calls.
//
// The motivating use case is the WASM warm-render bridge. Each call to
// boilerplateRenderFiles parses the bundle JSON and builds an fstest.MapFS
// — work that dominates the per-call latency on real bundles (~30-50 ms
// on a 500 KB / 94-file bundle). Callers that drive many renders against
// the same bundle within an editing session (the runbooks dispatcher,
// for example) can build a PreparedBundle once via boilerplatePrepareBundle
// and reuse it across every subsequent render, amortizing the parse cost
// to zero. The render-time cost then collapses to the per-file dep-tree
// walk that's inherently unavoidable.
//
// PreparedBundle is intentionally a thin convenience wrapper around the
// existing RenderFileFromFS / RenderFilesFromFS entry points so the
// renderer implementation stays single-sourced. The bridge stores
// PreparedBundle instances in a handle map keyed by an opaque ID it
// returns to JS; release frees the entry.
type PreparedBundle struct {
	// RootFS is the in-memory filesystem holding every bundle file,
	// keyed by forward-slash, strictly-relative paths from RootPath.
	RootFS fs.FS

	// RootPath is the directory inside RootFS the parent template lives
	// in. Empty / "." for the root-anchored bundles the WASM bridge
	// produces.
	RootPath string

	// DepsIndex is the producer-resolved dep tree the renderer walks.
	// MUST be non-nil — older bundles without this field are rejected
	// upstream by the bridge's structural validation so a nil reaches
	// no caller in practice.
	DepsIndex map[string][]ResolvedDep
}

// RenderFile renders a single output path against the prepared bundle.
// Semantically identical to RenderFileFromFS; the win is that the parse
// work the caller would otherwise repeat across calls is already done.
func (b *PreparedBundle) RenderFile(ctx context.Context, outputPath string, userVars map[string]any) (string, error) {
	return RenderFileFromFS(ctx, b.RootFS, b.RootPath, outputPath, userVars, b.DepsIndex)
}

// RenderFiles renders every path in outputPaths against the prepared
// bundle. Returns one entry per input path, in input order. Per-path
// failures are returned inline, same contract as RenderFilesFromFS.
func (b *PreparedBundle) RenderFiles(ctx context.Context, outputPaths []string, userVars map[string]any) []RenderFileResult {
	return RenderFilesFromFS(ctx, b.RootFS, b.RootPath, outputPaths, userVars, b.DepsIndex)
}
