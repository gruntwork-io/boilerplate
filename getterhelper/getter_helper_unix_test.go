//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package getterhelper_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/getterhelper"
	"github.com/gruntwork-io/boilerplate/pkg/logging"
	"github.com/gruntwork-io/boilerplate/pkg/vfs"
)

func TestDownloadTemplatesToTempDir(t *testing.T) {
	t.Parallel()

	templateURL := "git::https://github.com/gruntwork-io/boilerplate.git//examples/for-learning-and-testing/variables?ref=v0.12.1"

	workingDir, workPath, err := getterhelper.DownloadTemplatesToTemporaryFolder(logging.Discard(), vfs.NewOSFS(), templateURL)
	defer os.RemoveAll(workingDir)

	require.NoError(t, err)

	// Verify the download produced files
	entries, err := os.ReadDir(workPath)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
}
