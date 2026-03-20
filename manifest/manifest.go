// Package manifest provides functionality for reading and writing boilerplate generation manifests.
package manifest

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"

	"github.com/gruntwork-io/boilerplate/version"
)

const (
	// DefaultManifestFilename is the conventional filename used when writing a
	// manifest alongside generated output.
	DefaultManifestFilename = "boilerplate-manifest.yaml"

	// SchemaURL is the canonical URL of the current manifest JSON Schema.
	SchemaURL = "https://boilerplate.gruntwork.io/schemas/manifest/v1/schema.json"

	// SchemaVersion is the value written into the SchemaVersion field of every
	// manifest produced by this version of boilerplate. It currently equals
	// SchemaURL but is a separate constant so callers can distinguish between
	// "where the schema lives" and "which version this code emits".
	SchemaVersion = SchemaURL
)

const defaultFilePerm = 0o644

// Manifest represents the output manifest for a single boilerplate generation
// run. It captures the template that was used, the files that were generated,
// the variables that were supplied, and the dependencies that were processed.
type Manifest struct {
	Variables          map[string]any       `json:"Variables" yaml:"Variables" jsonschema:"required" jsonschema_description:"User-defined template variables used during generation"`
	SchemaVersion      string               `json:"SchemaVersion" yaml:"SchemaVersion" jsonschema:"required"`
	Timestamp          string               `json:"Timestamp" yaml:"Timestamp" jsonschema:"required"`
	TemplateURL        string               `json:"TemplateURL" yaml:"TemplateURL" jsonschema:"required"`
	BoilerplateVersion string               `json:"BoilerplateVersion" yaml:"BoilerplateVersion" jsonschema:"required"`
	SourceChecksum     string               `json:"SourceChecksum" yaml:"SourceChecksum" jsonschema:"required,pattern=^(git-sha1|git-sha256|sha256):.+$" jsonschema_description:"Checksum of the template source. For git sources: git-sha1:{commit} or git-sha256:{commit}. For local sources: sha256:{hex}."`
	OutputDir          string               `json:"OutputDir" yaml:"OutputDir" jsonschema:"required"`
	Dependencies       []ManifestDependency `json:"Dependencies" yaml:"Dependencies" jsonschema:"required"`
	Files              []GeneratedFile      `json:"Files" yaml:"Files" jsonschema:"required"`
}

// ManifestDependency represents a single dependency that was processed during a
// boilerplate run, including its resolved template URL, output folder, and any
// files it generated.
type ManifestDependency struct {
	Variables            map[string]any  `json:"Variables,omitempty" yaml:"Variables,omitempty"`
	Name                 string          `json:"Name" yaml:"Name" jsonschema:"required"`
	TemplateURL          string          `json:"TemplateURL" yaml:"TemplateURL" jsonschema:"required"`
	OutputFolder         string          `json:"OutputFolder" yaml:"OutputFolder" jsonschema:"required"`
	SourceChecksum       string          `json:"SourceChecksum,omitempty" yaml:"SourceChecksum,omitempty"`
	Skip                 string          `json:"Skip,omitempty" yaml:"Skip,omitempty"`
	ForEachReference     string          `json:"ForEachReference,omitempty" yaml:"ForEachReference,omitempty"`
	Files                []GeneratedFile `json:"Files,omitempty" yaml:"Files,omitempty"`
	VarFiles             []string        `json:"VarFiles,omitempty" yaml:"VarFiles,omitempty"`
	ForEach              []string        `json:"ForEach,omitempty" yaml:"ForEach,omitempty"`
	DontInheritVariables bool            `json:"DontInheritVariables,omitempty" yaml:"DontInheritVariables,omitempty"`
}

// GeneratedFile represents a single file produced by boilerplate, identified by
// its path relative to the output directory and a content checksum (e.g.
// "sha256:abcdef...").
type GeneratedFile struct {
	Path     string `json:"Path" yaml:"Path" jsonschema:"required"`
	Checksum string `json:"Checksum" yaml:"Checksum" jsonschema:"required,pattern=^[a-z0-9]+:.+$" jsonschema_description:"Hash of the file contents, prefixed with the algorithm (e.g. sha256:abcdef…)"`
}

// NewManifest creates a new Manifest populated with the current timestamp,
// the running boilerplate version, and the latest [SchemaVersion]. Callers
// supply the remaining fields that vary per generation run.
func NewManifest(templateURL, outputDir, sourceChecksum string, files []GeneratedFile, variables map[string]any, dependencies []ManifestDependency) *Manifest {
	return &Manifest{
		SchemaVersion:      SchemaVersion,
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
		TemplateURL:        templateURL,
		BoilerplateVersion: version.GetVersion(),
		SourceChecksum:     sourceChecksum,
		OutputDir:          outputDir,
		Files:              files,
		Variables:          variables,
		Dependencies:       dependencies,
	}
}

// ParseManifest parses a Manifest from raw bytes. The format (JSON or YAML)
// is auto-detected: if the data is valid JSON it is decoded as JSON, otherwise
// it is decoded as YAML.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest

	if json.Valid(data) {
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}

		return &m, nil
	}

	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

// ParseManifestFile reads and parses a Manifest from a file on disk. The
// format is auto-detected the same way as ParseManifest.
func ParseManifestFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseManifest(data)
}

// WriteManifest writes the manifest to the given path. The format (JSON or YAML)
// is auto-detected from the file extension: .json produces JSON, everything else
// produces YAML.
func WriteManifest(manifestPath string, m *Manifest) error {
	var (
		data []byte
		err  error
	)

	if isJSONExtension(manifestPath) {
		data, err = json.MarshalIndent(m, "", "  ")
	} else {
		data, err = yaml.Marshal(m)
	}

	if err != nil {
		return err
	}

	return os.WriteFile(manifestPath, data, defaultFilePerm)
}

// Validate validates raw manifest bytes against the JSON Schema that
// corresponds to the manifest's SchemaVersion field. The format (JSON or YAML)
// is auto-detected the same way as [ParseManifest]. This allows manifests
// produced by older versions of boilerplate to be validated against the schema
// they were written with. It returns an error if the data cannot be parsed, the
// schema version is unrecognised, or any schema violations are found.
func Validate(data []byte) error {
	m, err := ParseManifest(data)
	if err != nil {
		return fmt.Errorf("parsing manifest for validation: %w", err)
	}

	return validate(m)
}

// ValidateFile reads a manifest file from disk and validates it against the
// JSON Schema that corresponds to the manifest's SchemaVersion field. The
// format is auto-detected the same way as [ParseManifestFile].
func ValidateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return Validate(data)
}

func validate(m *Manifest) error {
	schemaLoader, ok := schemaRegistry[m.SchemaVersion]
	if !ok {
		return fmt.Errorf("unknown manifest schema version: %q", m.SchemaVersion)
	}

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshalling manifest for validation: %w", err)
	}

	result, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewBytesLoader(data))
	if err != nil {
		return fmt.Errorf("validating manifest: %w", err)
	}

	if result.Valid() {
		return nil
	}

	var buf strings.Builder

	fmt.Fprintf(&buf, "manifest failed schema validation (%s):", m.SchemaVersion)

	for _, desc := range result.Errors() {
		fmt.Fprintf(&buf, "\n  - %s", desc)
	}

	return fmt.Errorf("%s", buf.String())
}

// SHA256File computes the SHA256 checksum of a file and returns it as "sha256:<hex>".
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// GenerateSchema returns a [jsonschema.Schema] reflecting the current
// [Manifest] struct. The returned schema is suitable for programmatic
// inspection or further serialisation.
func GenerateSchema() *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		DoNotReference: true,
	}
	schema := reflector.Reflect(&Manifest{})
	schema.ID = jsonschema.ID(SchemaURL)
	schema.Title = "Boilerplate Manifest Schema"
	schema.Description = "Schema for boilerplate generation manifest"

	return schema
}

// GenerateSchemaJSON returns the canonical JSON encoding of the manifest schema.
// This is the authoritative output that the on-disk schema.json must match.
func GenerateSchemaJSON() ([]byte, error) {
	schema := GenerateSchema()

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	if err := enc.Encode(schema); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

//go:embed schemas/manifest/v1/schema.json
var schemaV1 []byte

// schemaRegistry maps SchemaVersion values to their pre-loaded JSON Schema.
// When a new schema version is published, embed its file and add an entry here.
var schemaRegistry = map[string]gojsonschema.JSONLoader{
	SchemaURL: gojsonschema.NewBytesLoader(schemaV1),
}

func isJSONExtension(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".json"
}
