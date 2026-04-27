package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/boilerplate/internal/shell"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
)

// ComputeSourceChecksum computes a checksum for the template source identified
// by templateDir and cloneDir. When cloneDir points to a git repository it
// delegates to [GitSourceChecksum]; otherwise (or if git detection fails) it
// falls back to [DirectorySourceChecksum].
func ComputeSourceChecksum(l logging.Logger, fsys vfs.FS, templateDir, cloneDir string) (string, error) {
	if cloneDir == "" {
		return DirectorySourceChecksum(fsys, templateDir)
	}

	gitDir := filepath.Join(cloneDir, ".git")

	info, err := fsys.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return DirectorySourceChecksum(fsys, templateDir)
	}

	checksum, err := GitSourceChecksum(l, cloneDir)
	if err != nil {
		l.Warnf("git source checksum failed, falling back to directory checksum: %v", err)

		return DirectorySourceChecksum(fsys, templateDir)
	}

	return checksum, nil
}

// GitSourceChecksum returns the HEAD commit hash of the git repository at
// cloneDir, prefixed with the object format: "git-sha1:<hash>" for
// SHA-1 repos or "git-sha256:<hash>" for SHA-256 repos.
func GitSourceChecksum(l logging.Logger, cloneDir string) (string, error) {
	format, err := shell.RunShellCommandAndGetOutput(l, cloneDir, nil, "git", "rev-parse", "--show-object-format")
	if err != nil {
		return "", fmt.Errorf("failed to detect git object format: %w", err)
	}

	format = strings.TrimSpace(format)

	hash, err := shell.RunShellCommandAndGetOutput(l, cloneDir, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	hash = strings.TrimSpace(hash)

	return fmt.Sprintf("git-%s:%s", format, hash), nil
}

// DirectorySourceChecksum computes a deterministic SHA-256 checksum over all
// regular files in dir on the given filesystem, skipping the .git directory.
// Files are walked in lexical order and each file contributes its relative path
// (forward-slash normalised) and contents to the hash. The returned string has
// the form "sha256:<hex>".
func DirectorySourceChecksum(fsys vfs.FS, dir string) (string, error) {
	h := sha256.New()

	err := vfs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if !d.Type().IsRegular() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes for cross-platform determinism.
		relPath = filepath.ToSlash(relPath)

		// Write path with null separator.
		if _, err := io.WriteString(h, relPath+"\x00"); err != nil {
			return err
		}

		return hashFileInto(fsys, path, h)
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// hashFileInto opens the file at path on the given filesystem and copies its
// contents into the provided hasher.
func hashFileInto(fsys vfs.FS, path string, h io.Writer) (err error) {
	f, err := fsys.Open(path)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	return nil
}
