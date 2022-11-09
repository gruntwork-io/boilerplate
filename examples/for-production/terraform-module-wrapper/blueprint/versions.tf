{{- $moduleInspectParsed := .UnderlyingModuleInspectJsonOutput | mustFromJson -}}
{{- $terraformConfigInspect := index $moduleInspectParsed "inspect-output" -}}

#---------------------------------------------------------------------------------------------------------------------
# CONFIGURE TERRAFORM AND PROVIDER VERSIONS
# ---------------------------------------------------------------------------------------------------------------------

terraform {
{{ if (index $terraformConfigInspect "required_core") }}required_version = "{{ index $terraformConfigInspect "required_core" | mustFirst }}"

{{ end -}}

  required_providers {
    {{ range $key, $provider := (index $terraformConfigInspect "required_providers") -}}
      {{ $key }} = {
        source  = "{{ index $provider "source" }}"
        {{ if (index $provider "version_constraints") }}version = "{{ index $provider "version_constraints" | mustFirst }}"
        {{ end -}}
      }
    {{ end -}}
  }
}