//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package render //nolint:testpackage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	zglob "github.com/mattn/go-zglob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/options"
)

func TestRenderJsonnet(t *testing.T) {
	t.Parallel()

	testFolders, err := zglob.Glob("../test-fixtures/jsonnet-unit-test/*")
	require.NoError(t, err)

	testBoilerplateOptions := &options.BoilerplateOptions{
		TemplateFolder: "/path/to/template",
		OutputFolder:   "/path/to/output",
	}

	for _, folder := range testFolders {
		testCaseName := filepath.Base(folder)
		templateFPath := filepath.Join(folder, "template.jsonnet")
		expectedFPath := filepath.Join(folder, "expected.json")
		variablesFPath := filepath.Join(folder, "variables.json")

		t.Run(testCaseName, func(t *testing.T) {
			t.Parallel()

			variablesJSON, err := os.ReadFile(variablesFPath)
			require.NoError(t, err)

			var variables map[string]any
			require.NoError(t, json.Unmarshal(variablesJSON, &variables))

			outputJSON, err := RenderJsonnetTemplate(templateFPath, variables, testBoilerplateOptions)
			require.NoError(t, err)
			var output map[string]any
			require.NoError(t, json.Unmarshal([]byte(outputJSON), &output))

			expectedOutputJSON, err := os.ReadFile(expectedFPath)
			require.NoError(t, err)
			var expectedOutput map[string]any
			require.NoError(t, json.Unmarshal(expectedOutputJSON, &expectedOutput))

			assert.Equal(t, expectedOutput, output)
		})
	}
}
