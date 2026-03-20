// Package v1 defines the v1 manifest schema types, validation, and schema
// generation.
//
// This package is frozen. The types, schema, and validation logic here must not
// be modified once published, because existing manifests on disk reference this
// schema version. If the manifest format needs to change, create a new version
// package (e.g. internal/manifest/v2) with its own types, embedded schema, and
// validation, then register it in the top-level manifest package's validator
// map.
package v1

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/xeipuuv/gojsonschema"
)

const (
	// SchemaURL is the canonical URL of the v1 manifest JSON Schema.
	SchemaURL = "https://boilerplate.gruntwork.io/schemas/manifest/v1/schema.json"

	// SchemaVersion is the version string written into manifests produced
	// under the v1 schema.
	SchemaVersion = SchemaURL
)

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

// Validate validates a v1 Manifest against the embedded JSON Schema.
func Validate(m *Manifest) error {
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

	fmt.Fprintf(&buf, "manifest failed schema validation (%s):", SchemaVersion)

	for _, desc := range result.Errors() {
		fmt.Fprintf(&buf, "\n  - %s", desc)
	}

	return fmt.Errorf("%s", buf.String())
}

// GenerateSchema returns a [jsonschema.Schema] reflecting the v1 [Manifest]
// struct. The returned schema is suitable for programmatic inspection or
// further serialisation.
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

// GenerateSchemaJSON returns the canonical JSON encoding of the v1 manifest
// schema. This is the authoritative output that the on-disk schema.json must
// match.
func GenerateSchemaJSON() ([]byte, error) {
	schema := GenerateSchema()

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")

	if err := enc.Encode(schema); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

//go:embed schema.json
var schemaJSON []byte

var schemaLoader = gojsonschema.NewBytesLoader(schemaJSON)
