//go:build !(js && wasm)

package fileutil

import (
	"os"

	"github.com/gabriel-vasile/mimetype"
)

const textMimeType = "text/plain"

// IsTextFile - usage of mimetype library to identify if the file is binary or text.
func IsTextFile(path string) (bool, error) {
	if !PathExists(path) {
		return false, NoSuchFile(path)
	}
	// consider empty file as binary file
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if fileInfo.Size() == 0 {
		return false, nil
	}

	detectedMIME, err := mimetype.DetectFile(path)
	if err != nil {
		return false, err
	}

	for mtype := detectedMIME; mtype != nil; mtype = mtype.Parent() {
		if mtype.Is(textMimeType) {
			return true, nil
		}
	}

	return false, nil
}
