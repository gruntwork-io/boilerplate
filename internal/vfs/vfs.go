// Package vfs provides a virtual filesystem abstraction for testing and production use.
// It wraps afero to provide a consistent interface for filesystem operations so that
// callers can substitute an in-memory filesystem during tests or run against the real
// operating system in production.
package vfs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/afero"
)

// FS is the filesystem interface used throughout the codebase.
// It provides an abstraction over real and in-memory filesystems.
type FS = afero.Fs

// File represents a file in the filesystem.
type File = afero.File

// HardLinker is an optional interface for filesystems that support hard links.
type HardLinker interface {
	LinkIfPossible(oldname, newname string) error
}

// ErrNoHardLink is returned when a filesystem does not support hard links.
var ErrNoHardLink = errors.New("hard link not supported")

// NewOSFS returns a filesystem backed by the real operating system filesystem.
func NewOSFS() FS {
	return &osFS{Fs: afero.NewOsFs()}
}

// NewMemMapFS returns an in-memory filesystem for testing purposes.
// The returned filesystem supports symlink operations via an in-memory link table.
func NewMemMapFS() FS {
	return &memMapFS{
		Fs:       afero.NewMemMapFs(),
		symlinks: make(map[string]string),
	}
}

// FileExists checks if a path exists using the given filesystem.
// Returns (true, nil) if the file exists, (false, nil) if it does not exist,
// and (false, error) for other errors (e.g., permission denied).
func FileExists(vfs FS, path string) (bool, error) {
	_, err := vfs.Stat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, err
}

// WriteFile writes data to a file on the given filesystem, creating any
// missing parent directories.
func WriteFile(fsys FS, filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if err := fsys.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	return afero.WriteFile(fsys, filename, data, perm)
}

// ReadFile reads the contents of a file from the given filesystem.
func ReadFile(fsys FS, filename string) ([]byte, error) {
	return afero.ReadFile(fsys, filename)
}

// MkdirTemp creates a temporary directory on the given filesystem.
func MkdirTemp(fsys FS, dir, pattern string) (string, error) {
	return afero.TempDir(fsys, dir, pattern)
}

// Link creates a hard link. It delegates to LinkIfPossible for filesystems
// that implement the HardLinker interface.
func Link(fsys FS, oldname, newname string) error {
	linker, ok := fsys.(HardLinker)
	if !ok {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: ErrNoHardLink}
	}

	return linker.LinkIfPossible(oldname, newname)
}

// Symlink creates a symbolic link. It uses afero's SymlinkIfPossible
// which is supported by OsFs and any FS implementing afero.Linker.
func Symlink(fsys FS, oldname, newname string) error {
	linker, ok := fsys.(afero.Linker)
	if !ok {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: afero.ErrNoSymlink}
	}

	return linker.SymlinkIfPossible(oldname, newname)
}

// WalkDir walks the file tree rooted at root, calling fn for each file or
// directory in the tree, including root. The fn callback receives an fs.DirEntry
// instead of os.FileInfo, which can be more efficient since it does not require
// a stat call for every visited file.
//
// All errors that arise visiting files and directories are filtered by fn:
// see the fs.WalkDirFunc documentation for details.
//
// The files are walked in lexical order, which makes the output deterministic
// but means that for very large directories WalkDir can be inefficient.
// WalkDir does not follow symbolic links.
//
// Adapted from spf13/afero#571 — replace with afero.WalkDir once merged.
func WalkDir(fsys FS, root string, fn fs.WalkDirFunc) error {
	info, err := lstatIfPossible(fsys, root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = walkDir(fsys, root, FileInfoDirEntry{FileInfo: info}, fn)
	}

	if errors.Is(err, filepath.SkipDir) || errors.Is(err, filepath.SkipAll) {
		return nil
	}

	return err
}

// osFS wraps afero.OsFs with hard link and symlink support.
type osFS struct {
	afero.Fs
}

// LinkIfPossible creates a hard link via os.Link.
func (fsys *osFS) LinkIfPossible(oldname, newname string) error {
	return os.Link(oldname, newname)
}

// SymlinkIfPossible creates a symbolic link via os.Symlink.
func (fsys *osFS) SymlinkIfPossible(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

// ReadlinkIfPossible reads the target of a symbolic link via os.Readlink.
func (fsys *osFS) ReadlinkIfPossible(name string) (string, error) {
	return os.Readlink(name)
}

// LstatIfPossible stats the named file without following symbolic links.
func (fsys *osFS) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	info, err := os.Lstat(name)

	return info, true, err
}

// memMapFS wraps afero.MemMapFs with in-memory symlink and hard-link support.
type memMapFS struct {
	afero.Fs
	symlinks map[string]string
}

// SymlinkIfPossible records a symbolic link in the in-memory link table.
func (fsys *memMapFS) SymlinkIfPossible(oldname, newname string) error {
	if _, exists := fsys.symlinks[newname]; exists {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: os.ErrExist}
	}

	fsys.symlinks[newname] = oldname

	return nil
}

// LinkIfPossible simulates a hard link by copying the source file's contents.
func (fsys *memMapFS) LinkIfPossible(oldname, newname string) error {
	if _, err := fsys.Fs.Stat(newname); err == nil {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: os.ErrExist}
	}

	data, err := afero.ReadFile(fsys.Fs, oldname)
	if err != nil {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: err}
	}

	info, err := fsys.Fs.Stat(oldname)
	if err != nil {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: err}
	}

	return afero.WriteFile(fsys.Fs, newname, data, info.Mode())
}

// ReadlinkIfPossible returns the target recorded in the in-memory link table.
func (fsys *memMapFS) ReadlinkIfPossible(name string) (string, error) {
	target, ok := fsys.symlinks[name]
	if !ok {
		return "", &os.PathError{Op: "readlink", Path: name, Err: os.ErrInvalid}
	}

	return target, nil
}

// LstatIfPossible reports whether the path is tracked as a symlink and falls
// back to Stat for regular entries.
func (fsys *memMapFS) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	if _, ok := fsys.symlinks[name]; ok {
		info, err := fsys.Fs.Stat(name)

		return info, true, err
	}

	info, err := fsys.Fs.Stat(name)

	return info, false, err
}

// FileInfoDirEntry wraps os.FileInfo to implement fs.DirEntry.
// Adapted from spf13/afero#571 — replace with afero equivalent once merged.
type FileInfoDirEntry struct {
	FileInfo os.FileInfo
}

// Name returns the base name of the file.
func (d FileInfoDirEntry) Name() string { return d.FileInfo.Name() }

// IsDir reports whether the entry describes a directory.
func (d FileInfoDirEntry) IsDir() bool { return d.FileInfo.IsDir() }

// Type returns the type bits of the file mode.
func (d FileInfoDirEntry) Type() fs.FileMode { return d.FileInfo.Mode().Type() }

// Info returns the FileInfo for the entry.
func (d FileInfoDirEntry) Info() (fs.FileInfo, error) { return d.FileInfo, nil }

// lstatIfPossible calls Lstat if the filesystem supports it, otherwise Stat.
func lstatIfPossible(fsys FS, path string) (os.FileInfo, error) {
	if lstater, ok := fsys.(afero.Lstater); ok {
		info, _, err := lstater.LstatIfPossible(path)
		return info, err
	}

	return fsys.Stat(path)
}

// walkDir recursively descends path, calling walkDirFn.
// Adapted from https://go.dev/src/path/filepath/path.go.
func walkDir(fsys FS, path string, d fs.DirEntry, walkDirFn fs.WalkDirFunc) error {
	if err := walkDirFn(path, d, nil); err != nil || !d.IsDir() {
		if errors.Is(err, filepath.SkipDir) && d.IsDir() {
			err = nil
		}

		return err
	}

	entries, err := readDirEntries(fsys, path)
	if err != nil {
		err = walkDirFn(path, d, err)
		if err != nil {
			if errors.Is(err, filepath.SkipDir) && d.IsDir() {
				err = nil
			}

			return err
		}
	}

	for _, entry := range entries {
		name := filepath.Join(path, entry.Name())
		if err := walkDir(fsys, name, entry, walkDirFn); err != nil {
			if errors.Is(err, filepath.SkipDir) {
				break
			}

			return err
		}
	}

	return nil
}

// readDirEntries reads the directory named by dirname and returns
// a sorted list of directory entries.
func readDirEntries(fsys FS, dirname string) (entries []fs.DirEntry, err error) {
	f, err := fsys.Open(dirname)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if rdf, ok := f.(fs.ReadDirFile); ok {
		entries, err = rdf.ReadDir(-1)
		if err != nil {
			return nil, err
		}

		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

		return entries, nil
	}

	infos, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	entries = make([]fs.DirEntry, len(infos))

	for i, info := range infos {
		entries[i] = FileInfoDirEntry{FileInfo: info}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	return entries, nil
}
