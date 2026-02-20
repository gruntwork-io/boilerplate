package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathExists returns true if the path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir returns true if the path points to a directory
func IsDir(path string) bool {
	fileInfo, err := os.Stat(path)
	return err == nil && fileInfo.IsDir()
}

// CopyFile copies a file from source to destination
func CopyFile(source string, destination string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	return WriteFileWithSamePermissions(source, destination, contents)
}

// WriteFileWithSamePermissions writes a file to the given destination with the given contents using the same permissions as the file at source
func WriteFileWithSamePermissions(source string, destination string, contents []byte) error {
	fileInfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	return os.WriteFile(destination, contents, fileInfo.Mode())
}

// CopyFolder copies all the files and folders in srcFolder to targetFolder.
func CopyFolder(srcFolder string, targetFolder string) error {
	return filepath.Walk(srcFolder, func(path string, info os.FileInfo, _ error) error {
		relPath, err := filepath.Rel(srcFolder, path)
		if err != nil {
			return err
		}

		if IsDir(path) {
			const defaultDirPerm = 0755
			return os.MkdirAll(filepath.Join(targetFolder, relPath), defaultDirPerm)
		} else {
			return CopyFile(path, filepath.Join(targetFolder, relPath))
		}
	})
}

// custom error types

type NoSuchFile string

func (path NoSuchFile) Error() string {
	return fmt.Sprintf("File %s does not exist", string(path))
}
