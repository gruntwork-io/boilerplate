package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/boilerplate/internal/logging"
	"github.com/gruntwork-io/boilerplate/internal/shell"
)

// ComputeSourceChecksum computes a checksum for the template source.
// For remote (git) sources, it returns the git commit SHA. For local sources,
// it computes a deterministic directory hash. If git detection fails, it falls
// back to directory hashing.
func ComputeSourceChecksum(templateDir, cloneDir string) (string, error) {
	if cloneDir == "" {
		return DirectorySourceChecksum(templateDir)
	}

	gitDir := filepath.Join(cloneDir, ".git")

	info, err := os.Stat(gitDir)
	if err != nil || !info.IsDir() {
		return DirectorySourceChecksum(templateDir)
	}

	checksum, err := GitSourceChecksum(cloneDir)
	if err != nil {
		logging.Logger.Printf("Warning: git source checksum failed, falling back to directory checksum: %v", err)

		return DirectorySourceChecksum(templateDir)
	}

	return checksum, nil
}

// GitSourceChecksum returns the HEAD commit hash of the git repo in cloneDir,
// prefixed with the object format (e.g. "git-sha1:<hash>" or "git-sha256:<hash>").
func GitSourceChecksum(cloneDir string) (string, error) {
	format, err := shell.RunShellCommandAndGetOutput(cloneDir, nil, "git", "rev-parse", "--show-object-format")
	if err != nil {
		return "", fmt.Errorf("failed to detect git object format: %w", err)
	}

	format = strings.TrimSpace(format)

	hash, err := shell.RunShellCommandAndGetOutput(cloneDir, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	hash = strings.TrimSpace(hash)

	return fmt.Sprintf("git-%s:%s", format, hash), nil
}

// DirectorySourceChecksum computes a deterministic SHA-256 checksum over all
// regular files in dir (skipping .git). Files are walked in lexical order;
// each file contributes its relative path and contents to the hash.
func DirectorySourceChecksum(dir string) (string, error) {
	h := sha256.New()

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
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

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(h, f); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
