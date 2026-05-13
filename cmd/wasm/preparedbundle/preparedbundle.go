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
// File layout: the pure-Go pieces (handle store, bundle parse) live in
// this file so they can be unit tested without a WASM build. The
// js.Func handlers, which use syscall/js, live in handlers_wasm.go
// behind the `js && wasm` build tag.
package preparedbundle

import (
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/inputs"
)

// bundleStore is the mutex-protected map of live handles. Keys are
// opaque monotonically-increasing strings produced by Store; values
// are *inputs.PreparedBundle, which is itself read-only after Store
// returns (so render-time access doesn't need the lock).
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
// can't depend on it.
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
// already-released handle is a no-op, matching the documented contract.
func (s *bundleStore) Release(id string) {
	s.mu.Lock()
	delete(s.bundles, id)
	s.mu.Unlock()
}

// parseBundle adapts bundlewasm.DecodeBundle into the PreparedBundle
// shape inputs.RenderFileFromFS / RenderFilesFromFS consume.
func parseBundle(bundleJSON string) (*inputs.PreparedBundle, error) {
	bundle, err := bundlewasm.DecodeBundle(bundleJSON)
	if err != nil {
		return nil, err
	}

	return &inputs.PreparedBundle{
		RootFS:    bundle.FS,
		RootPath:  bundle.RootPath,
		DepsIndex: bundle.Dependencies,
	}, nil
}
