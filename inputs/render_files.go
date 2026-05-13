package inputs

import (
	"context"
	"io/fs"
)

// RenderFileResult is one entry returned by RenderFilesFromFS. Exactly one of
// Content or Err is non-zero. Err may be a sentinel from render_file.go
// (discriminate with errors.Is) or a wrapped downstream error.
type RenderFileResult struct {
	Err     error
	Path    string
	Content string
}

// RenderFilesFromFS renders each path in outputPaths in input order. Per-path
// failures are returned inline so one broken template doesn't blank siblings.
func RenderFilesFromFS(ctx context.Context, rootFS fs.FS, rootPath string, outputPaths []string, userVars map[string]any, depsIndex map[string][]ResolvedDep) []RenderFileResult {
	results := make([]RenderFileResult, 0, len(outputPaths))

	for _, p := range outputPaths {
		content, err := RenderFileFromFS(ctx, rootFS, rootPath, p, userVars, depsIndex)
		results = append(results, RenderFileResult{Path: p, Content: content, Err: err})
	}

	return results
}
