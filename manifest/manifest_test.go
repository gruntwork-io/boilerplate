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

	"github.com/gruntwork-io/boilerplate/manifest"
)

const (
	// schemaPath is the path to the official published JSON Schema file, relative to this test file.
	schemaPath = "../docs/public/schemas/manifest/v1/schema.json"

	// embeddedSchemaPath is the path to the embedded copy used by the Validate function.
	embeddedSchemaPath = "schemas/manifest/v1/schema.json"
)

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

// TestEmbeddedSchemaMatchesPublicSchema verifies that the embedded copy of the
// schema (used by Validate) is byte-for-byte identical to the published copy in
// docs/public/. If this fails, copy the published schema into the embedded path.
func TestEmbeddedSchemaMatchesPublicSchema(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows due to CRLF line ending differences")
	}

	t.Parallel()

	publicAbs, err := filepath.Abs(schemaPath)
	require.NoError(t, err)

	public, err := os.ReadFile(publicAbs)
	require.NoError(t, err)

	embeddedAbs, err := filepath.Abs(embeddedSchemaPath)
	require.NoError(t, err)

	embedded, err := os.ReadFile(embeddedAbs)
	require.NoError(t, err)

	assert.Equal(t, string(public), string(embedded),
		"embedded schema does not match published schema in docs/public; copy the published file to %s", embeddedSchemaPath)
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

func TestParseManifest(t *testing.T) {
	t.Parallel()

	validJSON := `{
		"SchemaVersion": "` + manifest.SchemaVersion + `",
		"Timestamp": "2024-01-01T00:00:00Z",
		"TemplateURL": "my-template",
		"BoilerplateVersion": "v1.0.0",
		"SourceChecksum": "sha256:abc123",
		"OutputDir": "/output",
		"Variables": {"Name": "svc"},
		"Files": [{"Path": "a.txt", "Checksum": "sha256:aaa"}],
		"Dependencies": []
	}`

	validYAML := `SchemaVersion: "` + manifest.SchemaVersion + `"
Timestamp: "2024-01-01T00:00:00Z"
TemplateURL: my-template
BoilerplateVersion: v1.0.0
SourceChecksum: "sha256:abc123"
OutputDir: /output
Variables:
  Name: svc
Files:
  - Path: a.txt
    Checksum: "sha256:aaa"
Dependencies: []
`

	tests := []struct {
		name       string
		wantURL    string
		wantOutput string
		input      []byte
		wantFiles  int
		wantErr    bool
	}{
		{
			name:       "valid JSON",
			input:      []byte(validJSON),
			wantURL:    "my-template",
			wantOutput: "/output",
			wantFiles:  1,
		},
		{
			name:       "valid YAML",
			input:      []byte(validYAML),
			wantURL:    "my-template",
			wantOutput: "/output",
			wantFiles:  1,
		},
		{
			name:    "invalid content",
			input:   []byte(":::not valid yaml or json\t\t[[["),
			wantErr: true,
		},
		{
			name:       "empty manifest JSON",
			input:      []byte(`{}`),
			wantURL:    "",
			wantOutput: "",
			wantFiles:  0,
		},
		{
			name:       "JSON with dependencies",
			input:      []byte(`{"TemplateURL":"t","Dependencies":[{"Name":"vpc","TemplateURL":"./vpc","OutputFolder":"/out/vpc"}]}`),
			wantURL:    "t",
			wantOutput: "",
			wantFiles:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m, err := manifest.ParseManifest(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, m.TemplateURL)
			assert.Equal(t, tt.wantOutput, m.OutputDir)
			assert.Len(t, m.Files, tt.wantFiles)
		})
	}
}

func TestParseManifestFile(t *testing.T) {
	t.Parallel()

	validJSON := `{
		"SchemaVersion": "` + manifest.SchemaVersion + `",
		"Timestamp": "2024-01-01T00:00:00Z",
		"TemplateURL": "my-template",
		"BoilerplateVersion": "v1.0.0",
		"SourceChecksum": "sha256:abc123",
		"OutputDir": "/output",
		"Variables": {},
		"Files": [{"Path": "a.txt", "Checksum": "sha256:aaa"}],
		"Dependencies": []
	}`

	validYAML := `SchemaVersion: "` + manifest.SchemaVersion + `"
Timestamp: "2024-01-01T00:00:00Z"
TemplateURL: yaml-template
BoilerplateVersion: v1.0.0
SourceChecksum: "sha256:abc123"
OutputDir: /yaml-output
Variables: {}
Files:
  - Path: b.txt
    Checksum: "sha256:bbb"
Dependencies: []
`

	tests := []struct {
		name       string
		filename   string
		content    string
		wantURL    string
		wantOutput string
		wantErr    bool
	}{
		{
			name:       "JSON file",
			filename:   "manifest.json",
			content:    validJSON,
			wantURL:    "my-template",
			wantOutput: "/output",
		},
		{
			name:       "YAML file",
			filename:   "manifest.yaml",
			content:    validYAML,
			wantURL:    "yaml-template",
			wantOutput: "/yaml-output",
		},
		{
			name:       "YML extension",
			filename:   "manifest.yml",
			content:    validYAML,
			wantURL:    "yaml-template",
			wantOutput: "/yaml-output",
		},
		{
			name:     "nonexistent file",
			filename: "",
			wantErr:  true,
		},
		{
			name:     "invalid content on disk",
			filename: "bad.yaml",
			content:  ":::not valid\t\t[[[",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var filePath string
			if tt.filename == "" {
				filePath = filepath.Join(t.TempDir(), "does-not-exist.json")
			} else {
				filePath = filepath.Join(t.TempDir(), tt.filename)
				require.NoError(t, os.WriteFile(filePath, []byte(tt.content), 0o644))
			}

			m, err := manifest.ParseManifestFile(filePath)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, m.TemplateURL)
			assert.Equal(t, tt.wantOutput, m.OutputDir)
		})
	}
}

func TestParseManifestRoundTrip(t *testing.T) {
	t.Parallel()

	original := &manifest.Manifest{
		SchemaVersion:      manifest.SchemaVersion,
		Timestamp:          "2024-06-15T12:00:00Z",
		TemplateURL:        "round-trip-template",
		BoilerplateVersion: "v2.0.0",
		SourceChecksum:     "sha256:deadbeef",
		OutputDir:          "/round-trip",
		Variables:          map[string]any{"Key": "value"},
		Files:              []manifest.GeneratedFile{{Path: "f.txt", Checksum: "sha256:fff"}},
		Dependencies: []manifest.ManifestDependency{
			{Name: "dep1", TemplateURL: "./dep", OutputFolder: "/round-trip/dep"},
		},
	}

	tests := []struct {
		name     string
		filename string
	}{
		{name: "JSON round-trip", filename: "manifest.json"},
		{name: "YAML round-trip", filename: "manifest.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), tt.filename)
			require.NoError(t, manifest.WriteManifest(path, original))

			parsed, err := manifest.ParseManifestFile(path)
			require.NoError(t, err)

			assert.Equal(t, original.SchemaVersion, parsed.SchemaVersion)
			assert.Equal(t, original.Timestamp, parsed.Timestamp)
			assert.Equal(t, original.TemplateURL, parsed.TemplateURL)
			assert.Equal(t, original.BoilerplateVersion, parsed.BoilerplateVersion)
			assert.Equal(t, original.SourceChecksum, parsed.SourceChecksum)
			assert.Equal(t, original.OutputDir, parsed.OutputDir)
			assert.Equal(t, original.Files, parsed.Files)
			assert.Len(t, parsed.Dependencies, 1)
			assert.Equal(t, "dep1", parsed.Dependencies[0].Name)
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	validManifest := func() *manifest.Manifest {
		return &manifest.Manifest{
			SchemaVersion:      manifest.SchemaVersion,
			Timestamp:          "2024-01-01T00:00:00Z",
			TemplateURL:        "my-template",
			BoilerplateVersion: "v1.0.0",
			SourceChecksum:     "sha256:abc123",
			OutputDir:          "/output",
			Variables:          map[string]any{"Name": "svc"},
			Files:              []manifest.GeneratedFile{{Path: "a.txt", Checksum: "sha256:aaa"}},
			Dependencies:       []manifest.ManifestDependency{},
		}
	}

	tests := []struct {
		modify  func(m *manifest.Manifest)
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name:   "valid manifest",
			modify: func(m *manifest.Manifest) {},
		},
		{
			name: "unknown schema version",
			modify: func(m *manifest.Manifest) {
				m.SchemaVersion = "https://example.com/unknown/v99/schema.json"
			},
			wantErr: true,
			errMsg:  "unknown manifest schema version",
		},
		{
			name: "empty schema version",
			modify: func(m *manifest.Manifest) {
				m.SchemaVersion = ""
			},
			wantErr: true,
			errMsg:  "unknown manifest schema version",
		},
		{
			name: "missing required SourceChecksum",
			modify: func(m *manifest.Manifest) {
				m.SourceChecksum = ""
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "nil Files fails type check",
			modify: func(m *manifest.Manifest) {
				m.Files = nil
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "nil Dependencies fails type check",
			modify: func(m *manifest.Manifest) {
				m.Dependencies = nil
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "invalid SourceChecksum pattern",
			modify: func(m *manifest.Manifest) {
				m.SourceChecksum = "bad-no-prefix"
			},
			wantErr: true,
			errMsg:  "failed schema validation",
		},
		{
			name: "valid git-sha1 SourceChecksum",
			modify: func(m *manifest.Manifest) {
				m.SourceChecksum = "git-sha1:abc123def"
			},
		},
		{
			name: "valid git-sha256 SourceChecksum",
			modify: func(m *manifest.Manifest) {
				m.SourceChecksum = "git-sha256:abc123def"
			},
		},
		{
			name: "with dependencies",
			modify: func(m *manifest.Manifest) {
				m.Dependencies = []manifest.ManifestDependency{
					{Name: "vpc", TemplateURL: "./vpc", OutputFolder: "/out/vpc"},
				}
			},
		},
		{
			name: "NewManifest output validates",
			modify: func(_ *manifest.Manifest) {
				// replaced entirely in the test body
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var m *manifest.Manifest
			if tt.name == "NewManifest output validates" {
				m = manifest.NewManifest(
					"./tpl", "/out", "sha256:aaa",
					[]manifest.GeneratedFile{{Path: "f.txt", Checksum: "sha256:bbb"}},
					map[string]any{"k": "v"},
					[]manifest.ManifestDependency{},
				)
			} else {
				m = validManifest()
				tt.modify(m)
			}

			err := manifest.Validate(m)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)

				return
			}

			require.NoError(t, err)
		})
	}
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
