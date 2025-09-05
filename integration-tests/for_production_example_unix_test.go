//go:build aix || darwin || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd js,wasm linux netbsd openbsd solaris

package integrationtests_test

import (
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"

	"github.com/gruntwork-io/boilerplate/options"
)

func TestForProductionTerragruntArchitectureBoilerplateExample(t *testing.T) {
	t.Parallel()

	forProductionExamplePath := "../examples/for-production/terragrunt-architecture-catalog"

	outputBasePath := t.TempDir()
	// defer os.RemoveAll(outputBasePath)

	templateFolder, err := filepath.Abs(filepath.Join(forProductionExamplePath, "blueprints", "reference-architecture"))
	require.NoError(t, err)

	outputFolder := filepath.Join(outputBasePath, "infrastructure-live")
	varFile, err := filepath.Abs(filepath.Join(forProductionExamplePath, "sample_reference_architecture_vars.yml"))
	require.NoError(t, err)

	testExample(t, templateFolder, outputFolder, varFile, "", string(options.ExitWithError))

	// Make sure it rendered valid terragrunt outputs by running terragrunt validate in each of the relevant folders.
	t.Run("group", func(t *testing.T) {
		t.Parallel()

		for _, account := range []string{"dev", "stage", "prod"} {
			opts := &terraform.Options{
				TerraformBinary: "terragrunt",
				TerraformDir:    filepath.Join(outputFolder, account),
			}
			t.Run(account, func(t *testing.T) {
				t.Parallel()
				_, tfErr := terraform.RunTerraformCommandE(t, opts, terraform.FormatArgs(opts, "validate-all")...)
				require.NoError(t, tfErr)
			})
		}
	})
}
