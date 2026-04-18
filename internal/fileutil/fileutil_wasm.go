//go:build js && wasm

package fileutil

import (
	"bytes"
	"os"
	"unicode/utf8"
)

// IsTextFile uses a byte-level heuristic instead of the ~2MB mimetype
// signature table, which would dominate the WASM artifact. Empty files are
// reported as non-text to match the CLI build.
func IsTextFile(path string) (bool, error) {
	if !PathExists(path) {
		return false, NoSuchFile(path)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if fileInfo.Size() == 0 {
		return false, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false, err
	}

	sample := buf[:n]
	if bytes.IndexByte(sample, 0) != -1 {
		return false, nil
	}

	return utf8.Valid(sample), nil
}
