// Package preparedbundle exposes a JS handle API for parsing a bundle once
// and reusing the parse across many renders.
//
// JS contract (registered in cmd/wasm/full only):
//
//	boilerplatePrepareBundle(bundleJSON: string)
//	    -> handleID: string | Error
//	boilerplateRenderFilesWithHandle(handleID, pathsJSON, varsJSON)
//	    -> resultJSON: string | Error
//	boilerplateReleaseBundle(handleID) -> void
//
// A render against an unknown or already-released handle returns a structural
// Error so the caller can route the batch to cold render.
package preparedbundle

import (
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gruntwork-io/boilerplate/cmd/wasm/internal/bundlewasm"
	"github.com/gruntwork-io/boilerplate/inputs"
)

type bundleStore struct {
	bundles map[string]*inputs.PreparedBundle
	mu      sync.RWMutex
	nextID  atomic.Uint64
}

func newBundleStore() *bundleStore {
	return &bundleStore{bundles: make(map[string]*inputs.PreparedBundle)}
}

func (s *bundleStore) Store(b *inputs.PreparedBundle) string {
	id := "pb-" + strconv.FormatUint(s.nextID.Add(1), 10)

	s.mu.Lock()
	s.bundles[id] = b
	s.mu.Unlock()

	return id
}

// Get returns nil if the handle is unknown or released — callers must
// distinguish nil to surface a structural error rather than silently
// rendering against the wrong bundle.
func (s *bundleStore) Get(id string) *inputs.PreparedBundle {
	s.mu.RLock()
	b := s.bundles[id]
	s.mu.RUnlock()

	return b
}

// Release is idempotent.
func (s *bundleStore) Release(id string) {
	s.mu.Lock()
	delete(s.bundles, id)
	s.mu.Unlock()
}

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
