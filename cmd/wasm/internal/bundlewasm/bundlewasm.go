// Package bundlewasm holds the pure-Go helpers shared by every full-build
// WASM handler: the bundle JSON shape, path validation, MapFS construction,
// the inputs lifter, error-kind taxonomy, and per-handler boilerplate.
// syscall/js types live in the build-tagged sibling file so this package
// stays buildable on the host platform.
package bundlewasm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"strings"
	"testing/fstest"

	"github.com/gruntwork-io/boilerplate/inputs"
)

// Error-kind strings JS callers switch on.
const (
	KindOutputNotProduced    = "output_not_produced"
	KindDependencyNotBundled = "dependency_not_in_bundle"
	KindDynamicFilename      = "dynamic_filename"
	KindSkipFilesExcluded    = "skip_files_excluded"
	KindRender               = "render"
	KindStructural           = "structural"
)

// TemplateBundle is the JSON shape the JS bridge sends in. RootPath is the
// directory inside the bundle that holds the parent template's
// boilerplate.yml — "." for root-anchored bundles.
type TemplateBundle struct {
	Files        map[string]string               `json:"files"`
	Dependencies map[string][]inputs.ResolvedDep `json:"dependencies,omitempty"`
	RootPath     string                          `json:"rootPath"`
}

// Decoded is the validated, materialised form of TemplateBundle. The wire
// `Files` map has been transcribed into FS so the original strings can
// be reclaimed after DecodeBundle returns — meaningful at 500 KB bundles.
type Decoded struct {
	Dependencies map[string][]inputs.ResolvedDep
	FS           fs.FS
	RootPath     string
}

type PerFileError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// PerFileResult is one entry in the bulk-render result array. Exactly one
// of Content / Error is set; empty Content is a valid success.
type PerFileResult struct {
	Error   *PerFileError `json:"error,omitempty"`
	Path    string        `json:"path"`
	Content string        `json:"content,omitempty"`
}

type ResultPayload struct {
	Results []PerFileResult `json:"results"`
}

// ValidateBundlePath rejects paths that would let two keys refer to the
// same logical file, escape the bundle root, or use OS-specific separators
// the analyzer can't normalise.
func ValidateBundlePath(p string) error {
	if p == "" {
		return errors.New("empty path")
	}

	if strings.HasPrefix(p, "/") {
		return errors.New("absolute paths not allowed")
	}

	if strings.ContainsRune(p, '\\') {
		return errors.New("use forward slashes")
	}

	cleaned := path.Clean(p)
	if cleaned != p {
		return fmt.Errorf("non-canonical path; clean to %q", cleaned)
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return errors.New("path escapes bundle root")
	}

	return nil
}

// DecodeBundle parses bundleJSON, validates every recorded path, and
// materialises the file contents into an in-memory MapFS in the same pass.
// Empty RootPath is normalised to ".".
func DecodeBundle(bundleJSON string) (Decoded, error) {
	var bundle TemplateBundle
	if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
		return Decoded{}, fmt.Errorf("failed to parse bundle JSON: %w", err)
	}

	rootPath := bundle.RootPath
	if rootPath == "" {
		rootPath = "."
	}

	if rootPath != "." {
		if err := ValidateBundlePath(rootPath); err != nil {
			return Decoded{}, fmt.Errorf("invalid rootPath %q: %w", rootPath, err)
		}
	}

	mfs := make(fstest.MapFS, len(bundle.Files))
	for p, contents := range bundle.Files {
		if err := ValidateBundlePath(p); err != nil {
			return Decoded{}, fmt.Errorf("invalid bundle path %q: %w", p, err)
		}

		mfs[p] = &fstest.MapFile{Data: []byte(contents)}
	}

	for parent, siblings := range bundle.Dependencies {
		if parent != "." {
			if err := ValidateBundlePath(parent); err != nil {
				return Decoded{}, fmt.Errorf("invalid dependencies parent key %q: %w", parent, err)
			}
		}

		for _, dep := range siblings {
			if err := ValidateBundlePath(dep.BundlePath); err != nil {
				return Decoded{}, fmt.Errorf("invalid bundle path %q for dependency %q under %q: %w", dep.BundlePath, dep.Name, parent, err)
			}
		}
	}

	return Decoded{
		RootPath:     rootPath,
		Dependencies: bundle.Dependencies,
		FS:           mfs,
	}, nil
}

// ParseAndLiftVars unmarshals a JSON object into a map and lifts top-level
// "inputs" entries onto the root scope.
func ParseAndLiftVars(varsJSON string) (map[string]any, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(varsJSON), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse variables JSON: %w", err)
	}

	return LiftInputsToRoot(raw), nil
}

// ParseOutputPaths unmarshals a JSON-encoded string array and enforces
// non-emptiness — the contract the bulk-render handlers expose to JS.
func ParseOutputPaths(pathsJSON string) ([]string, error) {
	var outputPaths []string
	if err := json.Unmarshal([]byte(pathsJSON), &outputPaths); err != nil {
		return nil, fmt.Errorf("failed to parse outputPaths JSON: %w", err)
	}

	if len(outputPaths) == 0 {
		return nil, errors.New("outputPaths must be a non-empty array")
	}

	return outputPaths, nil
}

// LiftInputsToRoot hoists keys under "inputs" onto the root scope while
// preserving the "inputs" and "outputs" namespaces. Explicit root-scope
// entries win over lifted ones.
func LiftInputsToRoot(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}

	inputsBlock, _ := raw["inputs"].(map[string]any)
	if len(inputsBlock) == 0 {
		return raw
	}

	out := make(map[string]any, len(raw)+len(inputsBlock))
	maps.Copy(out, raw)

	for k, v := range inputsBlock {
		if _, exists := out[k]; exists {
			continue
		}

		out[k] = v
	}

	return out
}

// ClassifyError maps an error from inputs.RenderFileFromFS / RenderFiles
// to the short kind string JS callers switch on. Falls back to KindRender
// for anything that isn't a known sentinel.
func ClassifyError(err error) string {
	switch {
	case errors.Is(err, inputs.ErrOutputNotProduced):
		return KindOutputNotProduced
	case errors.Is(err, inputs.ErrDependencyNotInBundle):
		return KindDependencyNotBundled
	case errors.Is(err, inputs.ErrDynamicFilename):
		return KindDynamicFilename
	case errors.Is(err, inputs.ErrSkipFilesExcluded):
		return KindSkipFilesExcluded
	default:
		return KindRender
	}
}

// BuildResultPayload converts the per-path results from
// inputs.RenderFilesFromFS / PreparedBundle.RenderFiles into the
// JSON-serialisable shape the JS caller receives, classifying each error
// into the kind taxonomy.
func BuildResultPayload(raw []inputs.RenderFileResult) ResultPayload {
	payload := ResultPayload{Results: make([]PerFileResult, 0, len(raw))}

	for _, r := range raw {
		if r.Err != nil {
			kind := ClassifyError(r.Err)
			payload.Results = append(payload.Results, PerFileResult{
				Path: r.Path,
				Error: &PerFileError{
					Kind:    kind,
					Message: fmt.Sprintf("%s: %v", kind, r.Err),
				},
			})

			continue
		}

		payload.Results = append(payload.Results, PerFileResult{Path: r.Path, Content: r.Content})
	}

	return payload
}

// RecoverPanic is the deferred recover boilerplate every WASM handler
// uses. Call as `defer bundlewasm.RecoverPanic("renderFile")` — name is
// the handler tag that appears in the stderr message.
func RecoverPanic(name string) {
	if r := recover(); r != nil {
		fmt.Fprintln(os.Stderr, "boilerplate: recovered from panic in "+name+":", r)
	}
}
