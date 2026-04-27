package vfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gruntwork-io/boilerplate/pkg/vfs"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOSFS(t *testing.T) {
	t.Parallel()

	fsys := vfs.NewOSFS()

	assert.NotNil(t, fsys)

	_, ok := fsys.(afero.Symlinker)
	assert.True(t, ok, "expected OSFS to implement afero.Symlinker")

	_, ok = fsys.(vfs.HardLinker)
	assert.True(t, ok, "expected OSFS to implement vfs.HardLinker")
}

func TestNewMemMapFS(t *testing.T) {
	t.Parallel()

	fsys := vfs.NewMemMapFS()

	assert.NotNil(t, fsys)

	_, ok := fsys.(afero.Symlinker)
	assert.True(t, ok, "expected MemMapFS to implement afero.Symlinker")

	_, ok = fsys.(vfs.HardLinker)
	assert.True(t, ok, "expected MemMapFS to implement vfs.HardLinker")
}

func TestFileExists(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)
			existing := filepath.Join(root, "exists.txt")
			require.NoError(t, afero.WriteFile(fsys, existing, []byte("hi"), 0o644))

			ok, err := vfs.FileExists(fsys, existing)
			require.NoError(t, err)
			assert.True(t, ok)

			ok, err = vfs.FileExists(fsys, filepath.Join(root, "missing.txt"))
			require.NoError(t, err)
			assert.False(t, ok)
		})
	}
}

func TestWriteFileCreatesParents(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)
			target := filepath.Join(root, "nested", "deep", "file.txt")
			body := []byte("payload")

			require.NoError(t, vfs.WriteFile(fsys, target, body, 0o600))

			got, err := vfs.ReadFile(fsys, target)
			require.NoError(t, err)
			assert.Equal(t, body, got)
		})
	}
}

func TestMkdirTemp(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)

			dir, err := vfs.MkdirTemp(fsys, root, "vfs-")
			require.NoError(t, err)

			info, err := fsys.Stat(dir)
			require.NoError(t, err)
			assert.True(t, info.IsDir())
		})
	}
}

func TestSymlink(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)
			target := filepath.Join(root, "target.txt")
			require.NoError(t, afero.WriteFile(fsys, target, []byte("data"), 0o644))

			link := filepath.Join(root, "link.txt")
			require.NoError(t, vfs.Symlink(fsys, target, link))
		})
	}
}

func TestLink(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)
			src := filepath.Join(root, "src.txt")
			require.NoError(t, afero.WriteFile(fsys, src, []byte("data"), 0o644))

			dst := filepath.Join(root, "dst.txt")
			require.NoError(t, vfs.Link(fsys, src, dst))

			info, err := fsys.Stat(dst)
			require.NoError(t, err)
			assert.False(t, info.IsDir())
		})
	}
}

func TestWalkDirLexicalOrder(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)
			files := []string{
				filepath.Join(root, "b", "two.txt"),
				filepath.Join(root, "a", "one.txt"),
				filepath.Join(root, "a", "z.txt"),
			}

			for _, p := range files {
				require.NoError(t, vfs.WriteFile(fsys, p, []byte("x"), 0o644))
			}

			var visited []string

			err := vfs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() {
					return nil
				}

				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}

				visited = append(visited, filepath.ToSlash(rel))

				return nil
			})
			require.NoError(t, err)

			expected := []string{"a/one.txt", "a/z.txt", "b/two.txt"}
			assert.Equal(t, expected, visited)

			// Stable sort sanity: lexical order should match a sorted copy of itself.
			sorted := append([]string(nil), visited...)
			sort.Strings(sorted)
			assert.Equal(t, sorted, visited)
		})
	}
}

func TestWalkDirSkipDir(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)
			require.NoError(t, vfs.WriteFile(fsys, filepath.Join(root, "keep", "k.txt"), []byte("x"), 0o644))
			require.NoError(t, vfs.WriteFile(fsys, filepath.Join(root, "skip", "s.txt"), []byte("x"), 0o644))

			var visited []string

			err := vfs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() && filepath.Base(path) == "skip" {
					return filepath.SkipDir
				}

				if !d.IsDir() {
					rel, relErr := filepath.Rel(root, path)
					if relErr != nil {
						return relErr
					}

					visited = append(visited, filepath.ToSlash(rel))
				}

				return nil
			})
			require.NoError(t, err)

			assert.Equal(t, []string{"keep/k.txt"}, visited)
		})
	}
}

func TestWalkDirMissingRoot(t *testing.T) {
	t.Parallel()

	for _, backend := range fsBackends(t) {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()

			fsys, root := backend.new(t)

			var observed error

			err := vfs.WalkDir(fsys, filepath.Join(root, "does-not-exist"), func(_ string, _ fs.DirEntry, walkErr error) error {
				observed = walkErr
				return walkErr
			})
			require.Error(t, err)
			require.Error(t, observed)
			assert.ErrorIs(t, observed, fs.ErrNotExist)
		})
	}
}

type fsBackend struct {
	new  func(t *testing.T) (vfs.FS, string)
	name string
}

func fsBackends(t *testing.T) []fsBackend {
	t.Helper()

	return []fsBackend{
		{
			name: "osfs",
			new: func(t *testing.T) (vfs.FS, string) {
				t.Helper()
				return vfs.NewOSFS(), t.TempDir()
			},
		},
		{
			name: "memmapfs",
			new: func(t *testing.T) (vfs.FS, string) {
				t.Helper()

				fsys := vfs.NewMemMapFS()
				root := "/root"
				require.NoError(t, fsys.MkdirAll(root, os.ModePerm))

				return fsys, root
			},
		},
	}
}
