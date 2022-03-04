{{- define "base_envcommon_layout" -}}
# ---------------------------------------------------------------------------------------------------------------------
# COMMON TERRAGRUNT CONFIGURATION
# This is the common component configuration for {{ .EnvCommonComponent }}. The common variables for each environment to
# deploy {{ .EnvCommonComponent }} are defined here. This configuration will be merged into the environment configuration
# via an include block.
# ---------------------------------------------------------------------------------------------------------------------

# Terragrunt will copy the Terraform configurations specified by the source parameter, along with any files in the
# working directory, into a temporary folder, and execute your Terraform commands in that folder. If you're iterating
# locally, you can use --terragrunt-source /path/to/local/checkout/of/module to override the source parameter to a
# local check out of the module for faster iteration.
terraform {
  source = "${local.source_base_url}?ref={{ .RepoRef }}"
}

{{- if templateIsDefined "dependencies" }}

# ---------------------------------------------------------------------------------------------------------------------
# Dependencies are modules that need to be deployed before this one.
# ---------------------------------------------------------------------------------------------------------------------
{{ template "dependencies" . }}
{{- end }}

{{- if templateIsDefined "generate" }}

# ---------------------------------------------------------------------------------------------------------------------
# Generators are used to generate additional Terraform code that is necessary to deploy a module.
# ---------------------------------------------------------------------------------------------------------------------
{{ template "generate" . }}
{{- end }}

# ---------------------------------------------------------------------------------------------------------------------
# Locals are named constants that are reusable within the configuration.
# ---------------------------------------------------------------------------------------------------------------------
locals {
  source_base_url = "git::{{ .GruntworkGitBaseURLSSH }}/{{ .RepoName }}.git//{{ .ModulePath }}"

  {{- if .IncludeCommonVars }}

  # Automatically load common variables shared across all accounts
  common_vars = read_terragrunt_config(find_in_parent_folders("common.hcl"))

  # Extract the name prefix for easy access
  name_prefix = local.common_vars.locals.name_prefix
  {{- end }}

  {{- if .IncludeAccountVars }}

  # Automatically load account-level variables
  account_vars = read_terragrunt_config(find_in_parent_folders("account.hcl"))

  # Extract the account_name and account_id for easy access
  account_name = local.account_vars.locals.account_name
  account_id   = local.account_vars.locals.account_id
  {{- end }}

  {{- if .IncludeRegionVars }}

  # Automatically load region-level variables
  region_vars = read_terragrunt_config(find_in_parent_folders("region.hcl"))

  # Extract the region for easy access
  aws_region = local.region_vars.locals.aws_region
  {{- end }}
{{- if templateIsDefined "locals" }}
{{ template "locals" . }}
{{- end }}
}

# ---------------------------------------------------------------------------------------------------------------------
# MODULE PARAMETERS
# These are the variables we have to pass in to use the module specified in the terragrunt configuration above.
{{- if .IsEnvCommon }}
# This defines the parameters that are common across all environments.
{{- end }}
# ---------------------------------------------------------------------------------------------------------------------
inputs = {
  {{- template "inputs" . -}}
}
{{- end -}}
