package util

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gruntwork-io/boilerplate/errors"
)

const textMimeType = "text/plain"

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

// IsTextFile - usage of mimetype library to identify if the file is binary or text.
func IsTextFile(path string) (bool, error) {
	if !PathExists(path) {
		return false, NoSuchFile(path)
	}
	// consider empty file as binary file
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, errors.WithStackTrace(err)
	}

	if fileInfo.Size() == 0 {
		return false, nil
	}

	detectedMIME, err := mimetype.DetectFile(path)
	if err != nil {
		return false, errors.WithStackTrace(err)
	}

	for mtype := detectedMIME; mtype != nil; mtype = mtype.Parent() {
		if mtype.Is(textMimeType) {
			return true, nil
		}
	}

	return false, nil
}

// CommandInstalled returns true if the OS has the given command installed
func CommandInstalled(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// RunCommandAndGetOutput runs the given command and returns its stdout and stderr as a string
func RunCommandAndGetOutput(command string, args ...string) (string, error) {
	return RunCommandAndGetOutputWithContext(context.Background(), command, args...)
}

// RunCommandAndGetOutputWithContext runs the given command and returns its stdout and stderr as a string
func RunCommandAndGetOutputWithContext(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	bytes, err := cmd.Output()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return string(bytes), nil
}

// CopyFile copies a file from source to destination
func CopyFile(source string, destination string) error {
	contents, err := os.ReadFile(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	return WriteFileWithSamePermissions(source, destination, contents)
}

// WriteFileWithSamePermissions writes a file to the given destination with the given contents using the same permissions as the file at source
func WriteFileWithSamePermissions(source string, destination string, contents []byte) error {
	fileInfo, err := os.Stat(source)
	if err != nil {
		return errors.WithStackTrace(err)
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
