package inputs

import (
	"context"
	"io/fs"
)

// PreparedBundle is a pre-parsed bundle reusable across many render calls.
// It carries no per-render state.
type PreparedBundle struct {
	RootFS    fs.FS
	DepsIndex map[string][]ResolvedDep
	RootPath  string
}

func (b *PreparedBundle) RenderFile(ctx context.Context, outputPath string, userVars map[string]any) (string, error) {
	return RenderFileFromFS(ctx, b.RootFS, b.RootPath, outputPath, userVars, b.DepsIndex)
}

func (b *PreparedBundle) RenderFiles(ctx context.Context, outputPaths []string, userVars map[string]any) []RenderFileResult {
	return RenderFilesFromFS(ctx, b.RootFS, b.RootPath, outputPaths, userVars, b.DepsIndex)
}
