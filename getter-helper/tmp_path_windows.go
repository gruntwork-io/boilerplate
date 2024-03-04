//go:build windows
// +build windows

package getter_helper

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// getTempFolder on Windows will return the computed temporary folder in UNC path to avoid long path issues.
// See https://docs.microsoft.com/en-us/dotnet/standard/io/file-path-formats#unc-paths for more information on Windows Long
// Paths and UNC.
func getTempFolder() (string, error) {
	workingDir, err := ioutil.TempDir("", "boilerplate-cache*")
	if err != nil {
		return workingDir, err
	}
	return filepath.ToSlash(fixLongPath(workingDir)), err
}

// The following function is copied almost verbatim from
// https://github.com/golang/go/blob/master/src/os/path_windows.go
// with the following modifications:
//
// - Always convert to long form (this is so that it stays long form even after elements are appended to the path)
// - Reference functions in the stdlib as opposed to internal private funcs.
//
// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// fixLongPath returns the extended-length (\\?\-prefixed) form of
// path when needed, in order to avoid the default 260 character file
// path limit imposed by Windows. If path is not easily converted to
// the extended-length form (for example, if path is a relative path
// or contains .. elements), or is short enough, fixLongPath returns
// path unmodified.
//
// See https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
func fixLongPath(path string) string {
	// The extended form begins with \\?\, as in
	// \\?\c:\windows\foo.txt or \\?\UNC\server\share\foo.txt.
	// The extended form disables evaluation of . and .. path
	// elements and disables the interpretation of / as equivalent
	// to \. The conversion here rewrites / to \ and elides
	// . elements as well as trailing or duplicate separators. For
	// simplicity it avoids the conversion entirely for relative
	// paths or paths containing .. elements. For now,
	// \\server\share paths are not converted to
	// \\?\UNC\server\share paths because the rules for doing so
	// are less well-specified.
	if len(path) >= 2 && path[:2] == `\\` {
		// Don't canonicalize UNC paths.
		return path
	}
	if !filepath.IsAbs(path) {
		// Relative path
		return path
	}

	const prefix = `\\?`

	pathbuf := make([]byte, len(prefix)+len(path)+len(`\`))
	copy(pathbuf, prefix)
	n := len(path)
	r, w := 0, len(prefix)
	for r < n {
		switch {
		case os.IsPathSeparator(path[r]):
			// empty block
			r++
		case path[r] == '.' && (r+1 == n || os.IsPathSeparator(path[r+1])):
			// /./
			r++
		case r+1 < n && path[r] == '.' && path[r+1] == '.' && (r+2 == n || os.IsPathSeparator(path[r+2])):
			// /../ is currently unhandled
			return path
		default:
			pathbuf[w] = '\\'
			w++
			for ; r < n && !os.IsPathSeparator(path[r]); r++ {
				pathbuf[w] = path[r]
				w++
			}
		}
	}
	// A drive's root directory needs a trailing \
	if w == len(`\\?\c:`) {
		pathbuf[w] = '\\'
		w++
	}
	return string(pathbuf[:w])
}
