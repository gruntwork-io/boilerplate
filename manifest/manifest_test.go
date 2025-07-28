package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateVersionedManifestMultipleVersions(t *testing.T) {
	t.Parallel()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "manifest-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// First update
	err = UpdateVersionedManifest(tempDir, "template1", map[string]interface{}{"var1": "value1"}, []GeneratedFile{{Path: "file1.txt"}})
	require.NoError(t, err)

	// Wait a moment to ensure different timestamps
	time.Sleep(time.Second * 1)

	// Second update
	err = UpdateVersionedManifest(tempDir, "template2", map[string]interface{}{"var2": "value2"}, []GeneratedFile{{Path: "file2.txt"}})
	require.NoError(t, err)

	// Read manifest
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest VersionedManifest
	err = json.Unmarshal(data, &manifest)
	require.NoError(t, err)

	// Verify we have 2 versions
	assert.Len(t, manifest.Versions, 2)
	assert.NotEmpty(t, manifest.LatestVersion)

	// Verify latest version points to second update
	latestEntry := manifest.Versions[manifest.LatestVersion]
	assert.Equal(t, "template2", latestEntry.TemplateURL)
	assert.Equal(t, map[string]interface{}{"var2": "value2"}, latestEntry.Variables)
}

func TestUpdateVersionedManifestInvalidExistingFile(t *testing.T) {
	t.Parallel()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "manifest-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create invalid JSON file
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")
	err = os.WriteFile(manifestPath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	// Update should still work (creates new manifest)
	err = UpdateVersionedManifest(tempDir, "template", map[string]interface{}{}, []GeneratedFile{})
	assert.NoError(t, err)

	// Verify manifest was recreated
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest VersionedManifest
	err = json.Unmarshal(data, &manifest)
	assert.NoError(t, err)
	assert.Len(t, manifest.Versions, 1)
}
