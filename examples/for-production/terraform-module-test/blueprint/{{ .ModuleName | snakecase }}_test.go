package test

import (
	"fmt"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func Test{{ .ModuleName | camelcase }}(t *testing.T) {
	t.Parallel()

	// Uncomment the items below to skip certain parts of the test
	// os.Setenv("SKIP_setup", "true")
	// os.Setenv("SKIP_apply", "true")
	// os.Setenv("SKIP_validate", "true")
	// os.Setenv("SKIP_destroy", "true")

	// Copy the examples folder to a temp folder so we can run tests in parallel without them conflicting with each other
	testFolder := test_structure.CopyTerraformFolderToTemp(t, "{{ .RelativePathToRoot }}", "{{ .RelativePathFromRootToModule }}")
	// Store test scratch data in this folder
	workingDir := filepath.Join(".test-stages", t.Name())

	// Configure our terraform.Options struct
	test_structure.RunTestStage(t, "setup", func() {
		// Generate a unique ID so each test run is namespaced
		uniqueId := random.UniqueId()

		terraformOptions := &terraform.Options{
			TerraformDir: testFolder,
			Vars: map[string]interface{}{
				"{{ .InputVarToSet }}": fmt.Sprintf("test-%s", uniqueId),
			},
		}
		test_structure.SaveTerraformOptions(t, workingDir, terraformOptions)
	})

	terraformOptions := test_structure.LoadTerraformOptions(t, workingDir)

	// At the end of the test (due to the defer), run 'terraform destroy' to clean up anything the module deployed
	defer test_structure.RunTestStage(t, "destroy", func() {
		terraform.Destroy(t, terraformOptions)
	})

	// Run 'terraform apply' to deploy the module
	test_structure.RunTestStage(t, "apply", func() {
		terraform.InitAndApply(t, terraformOptions)
	})

	// Validate that our module worked
	test_structure.RunTestStage(t, "validate", func() {
		// Check that the randomly generated string includes the input var we passed in as a prefix
		exampleOutput := terraform.OutputRequired(t, terraformOptions, "{{ .OutputVarToCheck }}")
		require.Contains(t, exampleOutput, terraformOptions.Vars["{{ .InputVarToSet }}"])
	})
}