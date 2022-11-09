#---------------------------------------------------------------------------------------------------------------------
# CONFIGURE TERRAFORM AND PROVIDER VERSIONS
# ---------------------------------------------------------------------------------------------------------------------

terraform {
  required_version = "{{ .TerraformVersionConstraint }}"

  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "{{ .RandomProviderVersionConstraint }}"
    }
  }
}