// Package preparedbundle exposes the prepared-bundle handle API: a way
// for JS callers to parse a bundle once and reuse the parse across
// many subsequent renders. It is the performance counterpart to
// cmd/wasm/renderfiles, which parses the bundle JSON and rebuilds the
// fstest.MapFS on every call. For interactive editing surfaces that
// drive 10-30 consecutive renders against the same bundle (the
// runbooks dispatcher's per-keystroke loop being the motivating
// example), amortizing the parse work to zero is the difference
// between "responsive" and "instant" — at ~30-50 ms of parse + MapFS
// construction on a 500 KB / 94-file bundle, the saving on every call
// after the first is substantial.
//
// JS contract (registered in cmd/wasm/full only — the lite build is
// unchanged):
//
//	boilerplatePrepareBundle(bundleJSON: string)
//	    -> handleID: string | Error
//	boilerplateRenderFilesWithHandle(handleID, pathsJSON, varsJSON)
//	    -> resultJSON: string | Error
//	boilerplateReleaseBundle(handleID) -> void
//
// Handles are opaque strings produced by the bridge. Callers are
// expected to release the handle when the underlying bundle becomes
// stale (typically at runbook switch). A render against an unknown or
// already-released handle returns a structural Error with kind
// "structural" so the caller can route the batch to cold render.
//
// File layout: the pure-Go pieces (handle store, bundle parse,
// classify/validate helpers) live in this file so they can be unit
// tested without a WASM build. The js.Func handlers, which use
// syscall/js, live in handlers_wasm.go behind the `js && wasm` build
// tag.
package preparedbundle

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing/fstest"

	"github.com/gruntwork-io/boilerplate/inputs"
)

// templateBundle mirrors the shape used by cmd/wasm/renderfile and
// cmd/wasm/renderfiles. Duplicated rather than imported so the WASM
// packages stay independent — same convention the other two follow.
type templateBundle struct {
	RootPath     string                          `json:"rootPath"`
	Files        map[string]string               `json:"files"`
	Dependencies map[string][]inputs.ResolvedDep `json:"dependencies"`
}

// perFileError, perFileResult, and resultPayload mirror the response
// shape boilerplateRenderFiles emits. Identical taxonomy of kinds, so
// the consumer's per-kind dispatch keeps working whether they're
// calling the handle-less or handle-based bulk entry point.
type perFileError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type perFileResult struct {
	Path    string        `json:"path"`
	Content string        `json:"content,omitempty"`
	Error   *perFileError `json:"error,omitempty"`
}

type resultPayload struct {
	Results []perFileResult `json:"results"`
}

// bundleStore is the mutex-protected map of live handles. Keys are
// opaque monotonically-increasing strings produced by Store; values
// are *inputs.PreparedBundle, which is itself read-only after Store
// returns (so render-time access doesn't need the lock).
//
// The mutex is cheap insurance: today's WASM is single-threaded so
// there's no real contention, but a future multi-threaded WASM (or a
// Go-side test exercising the store from multiple goroutines) should
// not be able to race the map.
type bundleStore struct {
	mu      sync.Mutex
	nextID  atomic.Uint64
	bundles map[string]*inputs.PreparedBundle
}

func newBundleStore() *bundleStore {
	return &bundleStore{bundles: make(map[string]*inputs.PreparedBundle)}
}

// Store registers b and returns the opaque handle ID the JS caller
// holds onto. The ID format is intentionally undocumented so callers
// can't depend on it — today it's "pb-<n>" with n monotonic.
func (s *bundleStore) Store(b *inputs.PreparedBundle) string {
	id := "pb-" + strconv.FormatUint(s.nextID.Add(1), 10)

	s.mu.Lock()
	s.bundles[id] = b
	s.mu.Unlock()

	return id
}

// Get returns the prepared bundle for id, or nil if the handle is
// unknown or has been released. Callers must distinguish nil from a
// real bundle and surface a structural error to JS — silently
// rendering against the wrong (or no) bundle would be a worse failure
// mode.
func (s *bundleStore) Get(id string) *inputs.PreparedBundle {
	s.mu.Lock()
	b := s.bundles[id]
	s.mu.Unlock()

	return b
}

// Release removes the handle. Idempotent: releasing an unknown or
// already-released handle is a no-op, matching the documented contract
// (the JS caller doesn't need to track which handles it has already
// cleaned up).
func (s *bundleStore) Release(id string) {
	s.mu.Lock()
	delete(s.bundles, id)
	s.mu.Unlock()
}

// parseBundle does the structural-validation + MapFS construction that
// cmd/wasm/renderfiles does inline on every call. Centralised here so
// the prepare path can run it once and reuse the result across many
// renders. Returns either a fully populated *inputs.PreparedBundle or
// a structural error describing what was wrong with the input.
func parseBundle(bundleJSON string) (*inputs.PreparedBundle, error) {
	var bundle templateBundle
	if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
		return nil, fmt.Errorf("failed to parse bundle JSON: %w", err)
	}

	rootPath := bundle.RootPath
	if rootPath == "" {
		rootPath = "."
	}

	if rootPath != "." {
		if err := validateBundlePath(rootPath); err != nil {
			return nil, fmt.Errorf("invalid rootPath %q: %w", rootPath, err)
		}
	}

	mfs := fstest.MapFS{}
	for p, contents := range bundle.Files {
		if err := validateBundlePath(p); err != nil {
			return nil, fmt.Errorf("invalid bundle path %q: %w", p, err)
		}

		mfs[p] = &fstest.MapFile{Data: []byte(contents)}
	}

	for parent, siblings := range bundle.Dependencies {
		if parent != "." {
			if err := validateBundlePath(parent); err != nil {
				return nil, fmt.Errorf("invalid dependencies parent key %q: %w", parent, err)
			}
		}

		for _, dep := range siblings {
			if err := validateBundlePath(dep.BundlePath); err != nil {
				return nil, fmt.Errorf("invalid bundle path %q for dependency %q under %q: %w", dep.BundlePath, dep.Name, parent, err)
			}
		}
	}

	return &inputs.PreparedBundle{
		RootFS:    mfs,
		RootPath:  rootPath,
		DepsIndex: bundle.Dependencies,
	}, nil
}

// liftInputsToRoot mirrors the same helper in cmd/wasm/renderfile and
// cmd/wasm/renderfiles. Duplicated rather than imported to keep the
// WASM packages independent — same convention the others follow.
func liftInputsToRoot(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}

	out := make(map[string]any, len(raw)*2)
	for k, v := range raw {
		out[k] = v
	}

	inputsBlock, ok := raw["inputs"].(map[string]any)
	if !ok {
		return out
	}

	for k, v := range inputsBlock {
		if _, exists := out[k]; exists {
			continue
		}

		out[k] = v
	}

	return out
}

// classifyError maps an error from inputs.RenderFileFromFS /
// RenderFiles to the short kind string the JS caller switches on. Same
// taxonomy boilerplateRenderFiles uses; duplicated here to keep
// packages independent.
func classifyError(err error) string {
	switch {
	case errors.Is(err, inputs.ErrOutputNotProduced):
		return "output_not_produced"
	case errors.Is(err, inputs.ErrDependencyNotInBundle):
		return "dependency_not_in_bundle"
	case errors.Is(err, inputs.ErrDynamicFilename):
		return "dynamic_filename"
	case errors.Is(err, inputs.ErrSkipFilesExcluded):
		return "skip_files_excluded"
	default:
		return "render"
	}
}

// validateBundlePath duplicates the rule applied by cmd/wasm/inputs,
// cmd/wasm/renderfile, and cmd/wasm/renderfiles. Same forward-slash,
// strictly-relative, canonical-path rule.
func validateBundlePath(p string) error {
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
