//go:build !(js && wasm)

package fileutil

import (
	"errors"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
)

const textMimeType = "text/plain"

// IsTextFile uses the mimetype library to identify whether the file at the given path is text or binary.
func IsTextFile(fsys vfs.FS, path string) (isText bool, err error) {
	if !PathExists(fsys, path) {
		return false, NoSuchFile(path)
	}
	// consider empty file as binary file
	fileInfo, err := fsys.Stat(path)
	if err != nil {
		return false, err
	}

	if fileInfo.Size() == 0 {
		return false, nil
	}

	f, err := fsys.Open(path)
	if err != nil {
		return false, err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	detectedMIME, err := mimetype.DetectReader(f)
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
