package manifest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"

	"github.com/gruntwork-io/boilerplate/internal/manifest"
)

// schemaPath is the path to the official published JSON Schema file, relative to this test file.
const schemaPath = "../../docs/public/schemas/manifest/v1/schema.json"

func TestWriteManifestJSON(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")

	m := &manifest.Manifest{
		SchemaVersion:      manifest.SchemaVersion,
		Timestamp:          "2024-01-01T00:00:00Z",
		TemplateURL:        "template1",
		BoilerplateVersion: "v1.0.0",
		OutputDir:          "/output",
		Files: []manifest.GeneratedFile{
			{Path: "file1.txt", Checksum: "sha256:abc123"},
			{Path: "file2.txt", Checksum: "sha256:def456"},
		},
		Dependencies: []manifest.ManifestDependency{
			{Name: "vpc", TemplateURL: "./modules/vpc", OutputFolder: "/output/vpc"},
		},
	}

	err := manifest.WriteManifest(manifestPath, m)
	require.NoError(t, err)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, manifest.SchemaVersion, result.SchemaVersion)
	assert.Equal(t, "2024-01-01T00:00:00Z", result.Timestamp)
	assert.Equal(t, "template1", result.TemplateURL)
	assert.Equal(t, "v1.0.0", result.BoilerplateVersion)
	assert.Equal(t, "/output", result.OutputDir)
	assert.Len(t, result.Files, 2)
	assert.Equal(t, "file1.txt", result.Files[0].Path)
	assert.Equal(t, "sha256:abc123", result.Files[0].Checksum)
	assert.Len(t, result.Dependencies, 1)
	assert.Equal(t, "vpc", result.Dependencies[0].Name)
	assert.Equal(t, "./modules/vpc", result.Dependencies[0].TemplateURL)
}

func TestWriteManifestYAML(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.yaml")

	m := &manifest.Manifest{
		SchemaVersion:      manifest.SchemaVersion,
		Timestamp:          "2024-01-01T00:00:00Z",
		TemplateURL:        "template1",
		BoilerplateVersion: "v1.0.0",
		OutputDir:          "/output",
		Files: []manifest.GeneratedFile{
			{Path: "file1.txt", Checksum: "sha256:abc123"},
		},
	}

	err := manifest.WriteManifest(manifestPath, m)
	require.NoError(t, err)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = yaml.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, manifest.SchemaVersion, result.SchemaVersion)
	assert.Equal(t, "template1", result.TemplateURL)
	assert.Len(t, result.Files, 1)
	assert.Equal(t, "sha256:abc123", result.Files[0].Checksum)
}

func TestWriteManifestYMLExtension(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "manifest.yml")

	m := &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Timestamp:     "2024-01-01T00:00:00Z",
		Files:         []manifest.GeneratedFile{{Path: "a.txt", Checksum: "sha256:aaa"}},
	}

	err := manifest.WriteManifest(manifestPath, m)
	require.NoError(t, err)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var result manifest.Manifest

	err = yaml.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, manifest.SchemaVersion, result.SchemaVersion)
}

func TestWriteManifestOverwritesPrevious(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	manifestPath := filepath.Join(tempDir, "boilerplate-manifest.json")

	// Write first manifest
	m1 := &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Timestamp:     "2024-01-01T00:00:00Z",
		TemplateURL:   "template1",
		Files:         []manifest.GeneratedFile{{Path: "file1.txt", Checksum: "sha256:aaa"}},
	}
	err := manifest.WriteManifest(manifestPath, m1)
	require.NoError(t, err)

	// Write second manifest (overwrites)
	m2 := &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Timestamp:     "2024-01-02T00:00:00Z",
		TemplateURL:   "template2",
		Files:         []manifest.GeneratedFile{{Path: "file2.txt", Checksum: "sha256:bbb"}},
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

// TestGeneratedSchemaMatchesPublicSchema verifies that the schema generated from
// the Go struct is byte-for-byte identical to the official published schema file.
// If a field is added, removed, or changed in the struct, this test fails until
// the on-disk schema is regenerated via GenerateSchemaJSON.
func TestGeneratedSchemaMatchesPublicSchema(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows due to CRLF line ending differences")
	}

	t.Parallel()

	generated, err := manifest.GenerateSchemaJSON()
	require.NoError(t, err)

	absPath, err := filepath.Abs(schemaPath)
	require.NoError(t, err)

	onDisk, err := os.ReadFile(absPath)
	require.NoError(t, err)

	assert.Equal(t, string(onDisk), string(generated),
		"on-disk schema.json does not match GenerateSchemaJSON() output; regenerate it")
}

// validateManifestAgainstSchema marshals the manifest to JSON and validates it
// against the official on-disk schema file. This ensures the Go types and the
// published schema stay in sync.
func validateManifestAgainstSchema(t *testing.T, m *manifest.Manifest) {
	t.Helper()

	absSchema, err := filepath.Abs(schemaPath)
	require.NoError(t, err)

	schemaLoader := gojsonschema.NewReferenceLoader("file://" + absSchema)

	data, err := json.Marshal(m)
	require.NoError(t, err)

	documentLoader := gojsonschema.NewBytesLoader(data)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	require.NoError(t, err)

	for _, desc := range result.Errors() {
		t.Errorf("schema violation: %s", desc)
	}
}

func TestManifestConformsToSchema(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows due to file:// URL path differences")
	}

	t.Parallel()

	m := manifest.NewManifest(
		"./templates/service",
		"/output",
		"sha256:abc123",
		[]manifest.GeneratedFile{
			{Path: "main.go", Checksum: "sha256:aaa"},
		},
		map[string]any{"ServiceName": "my-service", "Port": 8080},
		[]manifest.ManifestDependency{
			{
				Name:         "vpc",
				TemplateURL:  "./modules/vpc",
				OutputFolder: "/output/vpc",
				Variables:    map[string]any{"cidr": "10.0.0.0/16"},
				Files: []manifest.GeneratedFile{
					{Path: "main.tf", Checksum: "sha256:bbb"},
				},
			},
			{
				Name:                 "logging",
				TemplateURL:          "./modules/logging",
				OutputFolder:         "/output/logging",
				Skip:                 "true",
				DontInheritVariables: true,
			},
			{
				Name:             "multi",
				TemplateURL:      "./modules/multi",
				OutputFolder:     "/output/multi",
				ForEach:          []string{"a", "b"},
				ForEachReference: "items",
				VarFiles:         []string{"extra.yml"},
			},
		},
	)

	validateManifestAgainstSchema(t, m)
}

func TestNewManifest(t *testing.T) {
	t.Parallel()

	files := []manifest.GeneratedFile{
		{Path: "a.txt", Checksum: "sha256:abc"},
	}

	vars := map[string]any{"Name": "test-service", "Port": 8080}

	deps := []manifest.ManifestDependency{
		{
			Name:         "vpc",
			TemplateURL:  "./modules/vpc",
			OutputFolder: "/output/vpc",
			Variables:    map[string]any{"cidr": "10.0.0.0/16"},
		},
	}

	m := manifest.NewManifest("my-template", "/output", "sha256:abc123", files, vars, deps)

	assert.Equal(t, manifest.SchemaVersion, m.SchemaVersion)
	assert.NotEmpty(t, m.Timestamp)
	assert.Equal(t, "my-template", m.TemplateURL)
	assert.NotEmpty(t, m.BoilerplateVersion)
	assert.Equal(t, "sha256:abc123", m.SourceChecksum)
	assert.Equal(t, "/output", m.OutputDir)
	assert.Equal(t, files, m.Files)
	assert.Equal(t, vars, m.Variables)
	assert.Equal(t, deps, m.Dependencies)
}
