{{- define "base_layout" -}}
# This is the configuration for Terragrunt, a thin wrapper for Terraform: https://terragrunt.gruntwork.io/

# Override the terraform source with the actual version we want to deploy.
terraform {
  source = "${include.envcommon.locals.source_base_url}?ref={{ .RepoRef }}"
}

# Include the root `terragrunt.hcl` configuration, which has settings common across all environments & components.
include "root" {
  path = find_in_parent_folders()
}

# Include the component configuration, which has settings that are common for the component across all environments
include "envcommon" {
  path = "${dirname(find_in_parent_folders())}/_envcommon/{{ .EnvCommonComponent }}.hcl"
  {{- if eq .EnvCommonMergeStrategy "deep" }}
  # Perform a deep merge so that we can reference dependencies in the override parameters.
  merge_strategy = "deep"
  {{- end }}
  # We want to reference the variables from the included config in this configuration, so we expose it.
  expose = true
}

{{- if templateIsDefined "includes" }}
{{ template "includes" . }}
{{- end }}

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

{{- if templateIsDefined "locals" }}

# ---------------------------------------------------------------------------------------------------------------------
# Locals are named constants that are reusable within the configuration.
# ---------------------------------------------------------------------------------------------------------------------
locals {
{{- template "locals" . }}
}
{{- end }}

{{- if templateIsDefined "inputs" }}

# ---------------------------------------------------------------------------------------------------------------------
# Module parameters to pass in. Note that these parameters are environment specific.
# ---------------------------------------------------------------------------------------------------------------------
inputs = {
  {{- template "inputs" . }}
}
{{- end }}
{{- end -}}
