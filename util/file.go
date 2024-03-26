package util

import (
	"fmt"
	"github.com/gruntwork-io/boilerplate/errors"
	"github.com/xyproto/binary"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// Return true if the path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Return true if the path points to a directory
func IsDir(path string) bool {
	fileInfo, err := os.Stat(path)
	return err == nil && fileInfo.IsDir()
}

// IsTextFile - usage of xyproto/binary library to identify if the file is binary or text.
func IsTextFile(path string) (bool, error) {
	isBinary, err := binary.File(path)
	if err != nil {
		return false, err
	}
	return !isBinary, nil
}

// Return true if the OS has the given command installed
func CommandInstalled(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// Run the given command return its stdout and stderr as a string
func RunCommandAndGetOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	bytes, err := cmd.Output()
	if err != nil {
		return "", errors.WithStackTrace(err)
	}

	return string(bytes), nil
}

// Copy a file from source to destination
func CopyFile(source string, destination string) error {
	contents, err := ioutil.ReadFile(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	return WriteFileWithSamePermissions(source, destination, contents)
}

// Write a file to the given destination with the given contents using the same permissions as the file at source
func WriteFileWithSamePermissions(source string, destination string, contents []byte) error {
	fileInfo, err := os.Stat(source)
	if err != nil {
		return errors.WithStackTrace(err)
	}

	return ioutil.WriteFile(destination, contents, fileInfo.Mode())
}

// Copy all the files and folders in srcFolder to targetFolder.
func CopyFolder(srcFolder string, targetFolder string) error {
	return filepath.Walk(srcFolder, func(path string, info os.FileInfo, err error) error {
		relPath, err := filepath.Rel(srcFolder, path)
		if err != nil {
			return err
		}

		if IsDir(path) {
			return os.MkdirAll(filepath.Join(targetFolder, relPath), 0755)
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
