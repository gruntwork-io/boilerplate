package v1_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/gruntwork-io/boilerplate/internal/manifest/v1"
)

// publishedSchemaPath is the path to the official published JSON Schema file,
// relative to this test file.
const publishedSchemaPath = "../../../docs/public/schemas/manifest/v1/schema.json"

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		modify  func(m *v1.Manifest)
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name:   "valid manifest",
			modify: func(m *v1.Manifest) {},
		},
		{
			name: "valid with dependencies",
			modify: func(m *v1.Manifest) {
				m.Dependencies = []v1.ManifestDependency{
					{Name: "vpc", TemplateURL: "./vpc", OutputFolder: "/out/vpc"},
				}
			},
		},
		{
			name: "valid git-sha1 checksum",
			modify: func(m *v1.Manifest) {
				m.SourceChecksum = "git-sha1:abc123def"
			},
		},
		{
			name: "valid git-sha256 checksum",
			modify: func(m *v1.Manifest) {
				m.SourceChecksum = "git-sha256:abc123def"
			},
		},
		{
			name: "empty SourceChecksum fails pattern",
			modify: func(m *v1.Manifest) {
				m.SourceChecksum = ""
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "invalid SourceChecksum pattern",
			modify: func(m *v1.Manifest) {
				m.SourceChecksum = "bad-no-prefix"
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "nil Files fails type check",
			modify: func(m *v1.Manifest) {
				m.Files = nil
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "nil Dependencies fails type check",
			modify: func(m *v1.Manifest) {
				m.Dependencies = nil
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := validManifest()
			tt.modify(m)

			err := v1.Validate(m)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestGenerateSchemaJSON(t *testing.T) {
	t.Parallel()

	data, err := v1.GenerateSchemaJSON()
	require.NoError(t, err)
	assert.True(t, json.Valid(data), "GenerateSchemaJSON must produce valid JSON")

	var parsed map[string]any

	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, v1.SchemaURL, parsed["$id"])
	assert.Equal(t, "Boilerplate Manifest Schema", parsed["title"])
}

func TestGenerateSchema(t *testing.T) {
	t.Parallel()

	schema := v1.GenerateSchema()
	assert.Equal(t, v1.SchemaURL, string(schema.ID))
	assert.Equal(t, "Boilerplate Manifest Schema", schema.Title)
	assert.Equal(t, "Schema for boilerplate generation manifest", schema.Description)
}

func TestGeneratedSchemaMatchesPublished(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows due to CRLF line ending differences")
	}

	t.Parallel()

	generated, err := v1.GenerateSchemaJSON()
	require.NoError(t, err)

	absPath, err := filepath.Abs(publishedSchemaPath)
	require.NoError(t, err)

	published, err := os.ReadFile(absPath)
	require.NoError(t, err)

	assert.Equal(t, string(published), string(generated),
		"published schema.json does not match GenerateSchemaJSON() output; regenerate it")
}

func TestEmbeddedSchemaMatchesPublished(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows due to CRLF line ending differences")
	}

	t.Parallel()

	embeddedAbs, err := filepath.Abs("schema.json")
	require.NoError(t, err)

	embedded, err := os.ReadFile(embeddedAbs)
	require.NoError(t, err)

	publishedAbs, err := filepath.Abs(publishedSchemaPath)
	require.NoError(t, err)

	published, err := os.ReadFile(publishedAbs)
	require.NoError(t, err)

	assert.Equal(t, string(published), string(embedded),
		"embedded schema.json does not match published schema in docs/public")
}

func TestConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://boilerplate.gruntwork.io/schemas/manifest/v1/schema.json", v1.SchemaURL)
	assert.Equal(t, v1.SchemaURL, v1.SchemaVersion)
}

func validManifest() *v1.Manifest {
	return &v1.Manifest{
		SchemaVersion:      v1.SchemaVersion,
		Timestamp:          "2024-01-01T00:00:00Z",
		TemplateURL:        "my-template",
		BoilerplateVersion: "v1.0.0",
		SourceChecksum:     "sha256:abc123",
		OutputDir:          "/output",
		Variables:          map[string]any{"Name": "svc"},
		Files:              []v1.GeneratedFile{{Path: "a.txt", Checksum: "sha256:aaa"}},
		Dependencies:       []v1.ManifestDependency{},
	}
}
