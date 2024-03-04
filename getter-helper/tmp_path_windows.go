//go:build windows
// +build windows

package getter_helper

import (
	"io/ioutil"
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
	return fixLongPath(workingDir), err
}

// fixLongPath converts a given file path to an extended-length path on Windows.
// This helps in avoiding the 260 character limit on traditional Windows paths.
// The function returns the path unmodified if it's already in extended-length form,
// if it's a relative path, or if it contains directory traversal elements like "..".
func fixLongPath(path string) string {
	// Check if the path is already in extended-length form or is a UNC path.
	if len(path) > 4 && path[:4] == `\\?\` {
		return path // Already in extended-length form.
	}
	if len(path) >= 2 && path[:2] == `\\` {
		return path // UNC path, don't modify.
	}

	// Convert to absolute path to ensure we're working with a complete path.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path // If conversion fails, return the original path.
	}

	// Avoid modifying paths that are not suitable for conversion.
	if !filepath.IsAbs(absPath) || containsDotDot(absPath) {
		return path
	}

	// Prepend the extended-length prefix to the absolute path.
	const prefix = `\\?\`
	extendedPath := prefix + absPath

	// Replace all slashes with backslashes in the path.
	return filepath.ToSlash(extendedPath)
}

// containsDotDot checks if the given path contains ".." which signifies directory traversal.
func containsDotDot(path string) bool {
	return filepath.Clean(path) != filepath.Clean(filepath.Join(path, "..", ".."))
}
