package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gruntwork-io/boilerplate/errors"
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

// There is no way to know for sure if a file is text or binary. The best we can do is use various heuristics to guess.
// The best set of heuristics is in the Unix/Linux file command, so we use that if it's available. Otherwise, we turn
// to Go's HTTP package. For more info, see: http://stackoverflow.com/q/16760378/483528
func IsTextFile(path string) (bool, error) {
	mimeType, err := GuessMimeType(path)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(mimeType, "text"), nil
}

// Guess the mime type for the given file using a variety of heuristics. Under the hood, uses the Unix/Linux file
// command, if available, and Go's HTTP package otherwise.
func GuessMimeType(path string) (string, error) {
	if PathExists(path) {
		if CommandInstalled("file") {
			return guessMimeTypeUsingFileCommand(path)
		} else {
			return guessMimeTypeUsingGoHttpPackage(path)
		}
	} else {
		return "", errors.WithStackTrace(NoSuchFile(path))
	}
}

// Use the Unix/Linux "file" command to determine the mime type. This performs a number of checks and tends to do a
// good job with most files.
func guessMimeTypeUsingFileCommand(path string) (string, error) {
	return RunCommandAndGetOutput("file", "-b", "--mime", path)
}

// Use a package built into Go for detecting the mime type of arbitrary content. In my experience, it doesn't work
// very well.
func guessMimeTypeUsingGoHttpPackage(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errors.WithStackTrace(err)
	}
	defer file.Close()

	first512Bytes := make([]byte, 512)
	numBytesRead, err := file.Read(first512Bytes)
	if err != nil && err != io.EOF {
		return "", errors.WithStackTrace(err)
	}

	// If it's an empty file, there is no real distinction, so default to "false", as there is not much processing
	// you can do on an empty file anyway
	if numBytesRead == 0 {
		return "", nil
	}

	return http.DetectContentType(first512Bytes), nil
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

// custom error types

type NoSuchFile string

func (path NoSuchFile) Error() string {
	return fmt.Sprintf("File %s does not exist", string(path))
}
