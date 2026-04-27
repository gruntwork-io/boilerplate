// Package manifest provides functionality for reading, writing, and validating
// boilerplate generation manifests.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	v1 "github.com/gruntwork-io/boilerplate/internal/manifest/v1"
	v2 "github.com/gruntwork-io/boilerplate/internal/manifest/v2"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
	"github.com/gruntwork-io/boilerplate/version"
)

const (
	// DefaultManifestFilename is the conventional filename used when writing a
	// manifest alongside generated output.
	DefaultManifestFilename = "boilerplate-manifest.yaml"

	// SchemaURL is the canonical URL of the current manifest JSON Schema.
	SchemaURL = v2.SchemaURL

	// SchemaVersion is the value written into the SchemaVersion field of every
	// manifest produced by this version of boilerplate. It currently equals
	// SchemaURL but is a separate constant so callers can distinguish between
	// "where the schema lives" and "which version this code emits".
	SchemaVersion = v2.SchemaVersion
)

// Type aliases for the current manifest version. These allow consumers to
// reference the types without importing the internal package directly.
type (
	// Manifest represents the output manifest for a single boilerplate
	// generation run.
	Manifest = v2.Manifest

	// ManifestDependency represents a single dependency that was processed
	// during a boilerplate run.
	ManifestDependency = v2.ManifestDependency

	// GeneratedFile represents a single file produced by boilerplate.
	GeneratedFile = v2.GeneratedFile
)

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

// ParseManifestFile reads and parses a Manifest from a file on the given filesystem. The
// format is auto-detected the same way as [ParseManifest].
func ParseManifestFile(fsys vfs.FS, path string) (*Manifest, error) {
	data, err := vfs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}

	return ParseManifest(data)
}

// WriteManifest writes the manifest to the given path on the given filesystem. The format
// (JSON or YAML) is auto-detected from the file extension: .json produces JSON, everything
// else produces YAML.
func WriteManifest(fsys vfs.FS, manifestPath string, m *Manifest) error {
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

	return vfs.WriteFile(fsys, manifestPath, data, defaultFilePerm)
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

	validate, ok := validators[m.SchemaVersion]
	if !ok {
		return fmt.Errorf("unknown manifest schema version: %q", m.SchemaVersion)
	}

	return validate(data)
}

// ValidateFile reads a manifest file from the given filesystem and validates it against the
// JSON Schema that corresponds to the manifest's SchemaVersion field. The format is
// auto-detected the same way as [ParseManifestFile].
func ValidateFile(fsys vfs.FS, path string) error {
	data, err := vfs.ReadFile(fsys, path)
	if err != nil {
		return err
	}

	return Validate(data)
}

// GenerateSchemaJSON returns the canonical JSON encoding of the current
// manifest schema. This is the authoritative output that the on-disk
// schema.json must match.
func GenerateSchemaJSON() ([]byte, error) {
	return v2.GenerateSchemaJSON()
}

// SHA256File computes the SHA256 checksum of a file on the given filesystem and returns
// it as "sha256:<hex>".
func SHA256File(fsys vfs.FS, path string) (checksum string, err error) {
	f, err := fsys.Open(path)
	if err != nil {
		return "", err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

const defaultFilePerm = 0o644

// validators maps SchemaVersion values to their version-specific validation
// function. When a new schema version is added, register its validator here.
var validators = map[string]func(data []byte) error{
	v1.SchemaURL: validateV1,
	v2.SchemaURL: validateV2,
}

func validateV1(data []byte) error {
	var m v1.Manifest

	if json.Valid(data) {
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing manifest for validation: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing manifest for validation: %w", err)
		}
	}

	return v1.Validate(&m)
}

func validateV2(data []byte) error {
	var m v2.Manifest

	if json.Valid(data) {
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing manifest for validation: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing manifest for validation: %w", err)
		}
	}

	return v2.Validate(&m)
}

func isJSONExtension(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".json"
}
