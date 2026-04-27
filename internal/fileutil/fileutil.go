// Package fileutil provides file-system utility functions.
package fileutil

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/gruntwork-io/boilerplate/pkg/vfs"
)

// PathExists returns true if the path exists on the given filesystem.
func PathExists(fsys vfs.FS, path string) bool {
	_, err := fsys.Stat(path)
	return err == nil
}

// IsDir returns true if the path points to a directory on the given filesystem.
func IsDir(fsys vfs.FS, path string) bool {
	fileInfo, err := fsys.Stat(path)
	return err == nil && fileInfo.IsDir()
}

// CopyFile copies a file from source to destination on the given filesystem.
func CopyFile(fsys vfs.FS, source string, destination string) error {
	contents, err := vfs.ReadFile(fsys, source)
	if err != nil {
		return err
	}

	return WriteFileWithSamePermissions(fsys, source, destination, contents)
}

// WriteFileWithSamePermissions writes a file to the given destination on the given filesystem with the given contents using the same permissions as the file at source.
func WriteFileWithSamePermissions(fsys vfs.FS, source string, destination string, contents []byte) error {
	fileInfo, err := fsys.Stat(source)
	if err != nil {
		return err
	}

	return vfs.WriteFile(fsys, destination, contents, fileInfo.Mode())
}

// CopyFolder copies all the files and folders in srcFolder to targetFolder on the given filesystem.
func CopyFolder(fsys vfs.FS, srcFolder string, targetFolder string) error {
	return vfs.WalkDir(fsys, srcFolder, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(srcFolder, path)
		if err != nil {
			return err
		}

		if IsDir(fsys, path) {
			const defaultDirPerm = 0755

			return fsys.MkdirAll(filepath.Join(targetFolder, relPath), defaultDirPerm)
		}

		return CopyFile(fsys, path, filepath.Join(targetFolder, relPath))
	})
}

// custom error types

// NoSuchFile is returned when an operation references a path that does not exist.
type NoSuchFile string

// Error implements the error interface for NoSuchFile.
func (path NoSuchFile) Error() string {
	return fmt.Sprintf("File %s does not exist", string(path))
}

// IsNotExist reports whether the given error indicates that a path does not exist.
func IsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
