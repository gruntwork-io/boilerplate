package manifest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/manifest"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
)

func TestDirectorySourceChecksum(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o644))

	checksum, err := manifest.DirectorySourceChecksum(dir)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(checksum, "sha256:"), "expected sha256 prefix, got %s", checksum)
	assert.Len(t, strings.TrimPrefix(checksum, "sha256:"), 64)
}

func TestDirectorySourceChecksum_Deterministic(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	for _, dir := range []string{dir1, dir2} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("bbb"), 0o644))
	}

	c1, err := manifest.DirectorySourceChecksum(dir1)
	require.NoError(t, err)

	c2, err := manifest.DirectorySourceChecksum(dir2)
	require.NoError(t, err)

	assert.Equal(t, c1, c2)
}

func TestDirectorySourceChecksum_SkipsGitDir(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	for _, dir := range []string{dir1, dir2} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644))
	}

	require.NoError(t, os.MkdirAll(filepath.Join(dir2, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, ".git", "HEAD"), []byte("ref: refs/heads/main"), 0o644))

	c1, err := manifest.DirectorySourceChecksum(dir1)
	require.NoError(t, err)

	c2, err := manifest.DirectorySourceChecksum(dir2)
	require.NoError(t, err)

	assert.Equal(t, c1, c2)
}

func TestGitSourceChecksum(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	gitCmd := func(args ...string) string {
		t.Helper()

		cmd := exec.CommandContext(context.Background(), "git", args...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, string(out))

		return strings.TrimSpace(string(out))
	}

	gitCmd("init")
	gitCmd("config", "user.email", "test@test.com")
	gitCmd("config", "user.name", "Test")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644))
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	expectedHash := gitCmd("rev-parse", "HEAD")

	checksum, err := manifest.GitSourceChecksum(logging.Discard(), dir)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(checksum, "git-sha1:") || strings.HasPrefix(checksum, "git-sha256:"),
		"unexpected prefix in %s", checksum)
	assert.True(t, strings.HasSuffix(checksum, expectedHash),
		"expected hash %s in checksum %s", expectedHash, checksum)
}

func TestComputeSourceChecksum_FallbackOnNonGit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644))

	checksum, err := manifest.ComputeSourceChecksum(logging.Discard(), dir, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(checksum, "sha256:"), "expected sha256 prefix, got %s", checksum)
}

func TestComputeSourceChecksum_FallbackOnNonGitCloneDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644))

	checksum, err := manifest.ComputeSourceChecksum(logging.Discard(), dir, dir)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(checksum, "sha256:"), "expected sha256 prefix, got %s", checksum)
}
