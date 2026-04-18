//go:build js && wasm

package fileutil

import (
	"bytes"
	"os"
	"unicode/utf8"
)

// IsTextFile is a lightweight WASM build of the mimetype-based helper. The
// gabriel-vasile/mimetype library ships a ~2MB signature table that would
// dominate the compressed WASM artifact, so we use a byte-level heuristic
// instead: a file is "text" if its first 512 bytes contain no NUL bytes and
// form valid UTF-8.
//
// Empty files are reported as NOT text to match the CLI build
// (fileutil_cli.go: "consider empty file as binary file"), so the template
// walker copies empty files verbatim in both builds.
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
