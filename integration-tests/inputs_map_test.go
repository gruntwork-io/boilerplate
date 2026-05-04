package integrationtests_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/cli"
)

// inputsMapResult mirrors the JSON shape of inputs.Result so this integration
// test can decode it without depending on the inputs package directly. (We
// could import the package, but doing so would couple this test to its
// internal types; using a local mirror keeps the contract explicit.)
type inputsMapResult struct {
	Inputs map[string]struct {
		Name        string   `json:"name"`
		DeclaredIn  string   `json:"declared_in"`
		Files       []string `json:"files"`
		Type        string   `json:"type"`
		Description string   `json:"description,omitempty"`
	} `json:"inputs"`

	Files map[string][]string `json:"files"`

	Errors []struct {
		Kind     string `json:"kind"`
		Template string `json:"template,omitempty"`
		Name     string `json:"name,omitempty"`
		File     string `json:"file,omitempty"`
		Message  string `json:"message,omitempty"`
	} `json:"errors"`
}

// TestInputsMap_TransitiveFixture drives the `boilerplate inputs map`
// subcommand against a small fixture (one root + one nested dependency that
// receives a parent variable via an explicit value expression). It validates
// that the JSON output contains the expected entries, including the inverse
// `files` index and at least one transitive edge.
func TestInputsMap_TransitiveFixture(t *testing.T) {
	t.Parallel()

	app := cli.CreateBoilerplateCli()

	var stdout, stderr bytes.Buffer
	app.Writer = &stdout
	app.ErrWriter = &stderr

	args := []string{
		"boilerplate", "inputs", "map",
		"--template-url", "../test-fixtures/inputs-test/transitive",
	}

	require.NoError(t, app.Run(args), "stderr=%s", stderr.String())

	var got inputsMapResult
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got), "stdout=%s", stdout.String())

	// Root declarations.
	require.Contains(t, got.Inputs, ".:Region")
	assert.Equal(t, "string", got.Inputs[".:Region"].Type)
	assert.Equal(t, ".", got.Inputs[".:Region"].DeclaredIn)

	require.Contains(t, got.Inputs, ".:ProjectName")

	// Transitive edge: root's Region reaches modules/vpc/main.tf via the
	// explicit `{{ .Region }}` expression in the dependency's variables block.
	assert.Contains(t, got.Inputs[".:Region"].Files, "modules/vpc/main.tf",
		"root.Region should reach modules/vpc/main.tf via the dependency value expression")

	// Child declaration.
	require.Contains(t, got.Inputs, "modules/vpc:AwsRegion")
	assert.Equal(t, "modules/vpc", got.Inputs["modules/vpc:AwsRegion"].DeclaredIn)
	assert.Contains(t, got.Inputs["modules/vpc:AwsRegion"].Files, "modules/vpc/main.tf")

	// Inverse index covers both inputs that affect modules/vpc/main.tf.
	require.Contains(t, got.Files, "modules/vpc/main.tf")
	assert.Contains(t, got.Files["modules/vpc/main.tf"], ".:Region")
	assert.Contains(t, got.Files["modules/vpc/main.tf"], "modules/vpc:AwsRegion")

	// README.md references both root vars directly.
	require.Contains(t, got.Files, "README.md")
	assert.ElementsMatch(t,
		[]string{".:ProjectName", ".:Region"},
		got.Files["README.md"],
	)

	// No soft errors expected from this clean fixture.
	assert.Empty(t, got.Errors, "unexpected analysis errors: %+v", got.Errors)
}
