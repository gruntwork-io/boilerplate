package manifest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/gruntwork-io/boilerplate/manifest"
)

func TestWriteManifestJSON(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")

	m := &manifest.Manifest{
		SchemaVersion:      "1.0",
		Timestamp:          "2024-01-01T00:00:00Z",
		TemplateURL:        "template1",
		BoilerplateVersion: "v1.0.0",
		OutputDir:          "/output",
		Files: []manifest.GeneratedFile{
			{Path: "file1.txt", Checksum: "abc123"},
			{Path: "file2.txt", Checksum: "def456"},
		},
	}

	err := manifest.WriteManifest(manifestPath, m)
	require.NoError(t, err)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "1.0", result.SchemaVersion)
	assert.Equal(t, "2024-01-01T00:00:00Z", result.Timestamp)
	assert.Equal(t, "template1", result.TemplateURL)
	assert.Equal(t, "v1.0.0", result.BoilerplateVersion)
	assert.Equal(t, "/output", result.OutputDir)
	assert.Len(t, result.Files, 2)
	assert.Equal(t, "file1.txt", result.Files[0].Path)
	assert.Equal(t, "abc123", result.Files[0].Checksum)
}

func TestWriteManifestYAML(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.yaml")

	m := &manifest.Manifest{
		SchemaVersion:      "1.0",
		Timestamp:          "2024-01-01T00:00:00Z",
		TemplateURL:        "template1",
		BoilerplateVersion: "v1.0.0",
		OutputDir:          "/output",
		Files: []manifest.GeneratedFile{
			{Path: "file1.txt", Checksum: "abc123"},
		},
	}

	err := manifest.WriteManifest(manifestPath, m)
	require.NoError(t, err)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = yaml.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "1.0", result.SchemaVersion)
	assert.Equal(t, "template1", result.TemplateURL)
	assert.Len(t, result.Files, 1)
	assert.Equal(t, "abc123", result.Files[0].Checksum)
}

func TestWriteManifestYMLExtension(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "manifest.yml")

	m := &manifest.Manifest{
		SchemaVersion: "1.0",
		Timestamp:     "2024-01-01T00:00:00Z",
		Files:         []manifest.GeneratedFile{{Path: "a.txt", Checksum: "aaa"}},
	}

	err := manifest.WriteManifest(manifestPath, m)
	require.NoError(t, err)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = yaml.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "1.0", result.SchemaVersion)
}

func TestWriteManifestOverwritesPrevious(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")

	// Write first manifest
	m1 := &manifest.Manifest{
		SchemaVersion: "1.0",
		Timestamp:     "2024-01-01T00:00:00Z",
		TemplateURL:   "template1",
		Files:         []manifest.GeneratedFile{{Path: "file1.txt", Checksum: "aaa"}},
	}
	err := manifest.WriteManifest(manifestPath, m1)
	require.NoError(t, err)

	// Write second manifest (overwrites)
	m2 := &manifest.Manifest{
		SchemaVersion: "1.0",
		Timestamp:     "2024-01-02T00:00:00Z",
		TemplateURL:   "template2",
		Files:         []manifest.GeneratedFile{{Path: "file2.txt", Checksum: "bbb"}},
	}
	err = manifest.WriteManifest(manifestPath, m2)
	require.NoError(t, err)

	// Read back and verify it's the second manifest
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "template2", result.TemplateURL)
	assert.Equal(t, "2024-01-02T00:00:00Z", result.Timestamp)
	assert.Len(t, result.Files, 1)
	assert.Equal(t, "file2.txt", result.Files[0].Path)
}

func TestWriteManifestErrorsOnCorruptExistingFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")

	// Create a corrupted file
	err := os.WriteFile(manifestPath, []byte("invalid json"), 0o644)
	require.NoError(t, err)

	m := &manifest.Manifest{
		SchemaVersion: "1.0",
		Files:         []manifest.GeneratedFile{},
	}

	err = manifest.WriteManifest(manifestPath, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupted")
}

func TestNewManifest(t *testing.T) {
	t.Parallel()

	files := []manifest.GeneratedFile{
		{Path: "a.txt", Checksum: "abc"},
	}

	m := manifest.NewManifest("my-template", "/output", files)

	assert.Equal(t, manifest.SchemaVersion, m.SchemaVersion)
	assert.NotEmpty(t, m.Timestamp)
	assert.Equal(t, "my-template", m.TemplateURL)
	assert.NotEmpty(t, m.BoilerplateVersion)
	assert.Equal(t, "/output", m.OutputDir)
	assert.Equal(t, files, m.Files)
}
