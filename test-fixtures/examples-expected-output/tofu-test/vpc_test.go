//go:build ignore
// +build ignore

package test

import (
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

func TestVpc(t *testing.T) {
	t.Parallel()

	opts := &terraform.Options{
		TerraformDir:    "../examples/vpc",
		TerraformBinary: "tofu",
	}

	defer terraform.Destroy(t, opts)

	terraform.InitAndApply(t, opts)

	actualOutput := terraform.OutputRequired(t, opts, "example_output")
	assert.Equal(t, "Hello World", actualOutput)
}
