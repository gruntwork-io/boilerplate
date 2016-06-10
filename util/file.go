package util

import (
	"os"
	"strings"
	"net/http"
	"github.com/gruntwork-io/boilerplate/errors"
	"io/ioutil"
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

// Return true if the file at the given path is a "text file" as opposed to a "binary file". See here for details on
// how this works: https://groups.google.com/forum/#!topic/golang-nuts/YeLL7L7SwWs
func IsTextFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, errors.WithStackTrace(err)
	}
	defer file.Close()

	first512Bytes := make([]byte, 512)
	numBytesRead, err := file.Read(first512Bytes)
	if err != nil {
		return false, errors.WithStackTrace(err)
	}
	if numBytesRead == 0 {
		return false, nil
	}

	mimeType := http.DetectContentType(first512Bytes)
	return strings.HasPrefix(mimeType, "text"), nil
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