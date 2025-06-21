package manifest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManifest(t *testing.T) {
	t.Parallel()

	outputDir := "/test/output"

	manifest := NewManifest(outputDir)

	assert.Equal(t, outputDir, manifest.OutputDir)
	assert.Empty(t, manifest.Files)
}

func TestAddFile(t *testing.T) {
	t.Parallel()

	outputDir := "/test/output"
	manifest := NewManifest(outputDir)

	// Test adding a file in the output directory
	outputPath := filepath.Join(outputDir, "subdir", "file.txt")

	err := manifest.AddFile(outputPath)

	assert.NoError(t, err)
	assert.Len(t, manifest.Files, 1)
	assert.Equal(t, "subdir/file.txt", manifest.Files[0].Path)
}

func TestAddFileOutsideOutputDir(t *testing.T) {
	t.Parallel()

	outputDir := "/test/output"
	manifest := NewManifest(outputDir)

	// Test adding a file outside the output directory
	outputPath := "/other/dir/file.txt"

	err := manifest.AddFile(outputPath)

	assert.NoError(t, err)
	assert.Len(t, manifest.Files, 1)
	expectedRelPath, _ := filepath.Rel(outputDir, outputPath)
	assert.Equal(t, expectedRelPath, manifest.Files[0].Path)
}

func TestAddMultipleFiles(t *testing.T) {
	t.Parallel()

	outputDir := "/test/output"
	manifest := NewManifest(outputDir)

	// Add first file
	err1 := manifest.AddFile(
		filepath.Join(outputDir, "file1.txt"),
	)

	// Add second file
	err2 := manifest.AddFile(
		filepath.Join(outputDir, "subdir", "file2.txt"),
	)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Len(t, manifest.Files, 2)
	assert.Equal(t, "file1.txt", manifest.Files[0].Path)
	assert.Equal(t, "subdir/file2.txt", manifest.Files[1].Path)
}
