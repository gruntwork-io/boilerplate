package render

import (
	"encoding/json"
	"io/ioutil"
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

			variablesJson, err := ioutil.ReadFile(variablesFPath)
			require.NoError(t, err)

			var variables map[string]interface{}
			require.NoError(t, json.Unmarshal(variablesJson, &variables))

			outputJson, err := RenderJsonnetTemplate(templateFPath, variables, testBoilerplateOptions)
			require.NoError(t, err)
			var output map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(outputJson), &output))

			expectedOutputJson, err := ioutil.ReadFile(expectedFPath)
			require.NoError(t, err)
			var expectedOutput map[string]interface{}
			require.NoError(t, json.Unmarshal(expectedOutputJson, &expectedOutput))

			assert.Equal(t, expectedOutput, output)
		})
	}
}
