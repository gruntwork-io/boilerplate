package inputs

import (
	"context"
	"io/fs"
)

// RenderFileResult is one entry returned by RenderFilesFromFS. Exactly one
// of Content or Err is non-zero. Err is either one of the sentinel errors
// declared in render_file.go (ErrOutputNotProduced, ErrDependencyNotInBundle,
// ErrDynamicFilename, ErrSkipFilesExcluded — discriminate with errors.Is)
// or a wrapped error from a downstream call, which the WASM boundary
// classifies as the generic "render" kind.
type RenderFileResult struct {
	Path    string
	Content string
	Err     error
}

// RenderFilesFromFS renders each path in outputPaths against the same
// rootFS, rootPath, userVars, and depsIndex. Returns one entry per input
// path, in input order. Per-path failures are returned inline — a failed
// render does NOT abort sibling renders, so a single broken template
// can't blank a preview pane that's rendering N files at once.
//
// Semantically identical to calling RenderFileFromFS in a loop; exposed
// as a single call so consumers (notably the WASM bulk handler) can
// amortize their own per-call setup work — bundle JSON parse, MapFS
// construction, vars parse — across paths. The dep-tree walk itself
// still happens per path; the spec's headline win is the parse work the
// caller no longer repeats, not the walk itself.
func RenderFilesFromFS(ctx context.Context, rootFS fs.FS, rootPath string, outputPaths []string, userVars map[string]any, depsIndex map[string][]ResolvedDep) []RenderFileResult {
	results := make([]RenderFileResult, 0, len(outputPaths))

	for _, p := range outputPaths {
		content, err := RenderFileFromFS(ctx, rootFS, rootPath, p, userVars, depsIndex)
		results = append(results, RenderFileResult{Path: p, Content: content, Err: err})
	}

	return results
}
