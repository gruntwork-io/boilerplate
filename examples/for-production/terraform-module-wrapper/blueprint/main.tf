{{- $moduleInspectParsed := .UnderlyingModuleInspectJsonOutput | mustFromJson -}}
{{- $terraformConfigInspect := index $moduleInspectParsed "inspect-output" -}}

#---------------------------------------------------------------------------------------------------------------------
# CREATE THE WRAPPER MODULE
# ---------------------------------------------------------------------------------------------------------------------

module "{{ .WrapperModuleName | snakecase }}" {
  source = "{{ .UnderlyingModuleSourceUrl }}"

  {{- if $.ExcludeInputVars }}

    # Override these input variables with specific values
    {{ range $key, $variable := (index $terraformConfigInspect "variables") }}
      {{- if (has (index $variable "name") $.ExcludeInputVars) }}{{ index $variable "name" }} = null # TODO: override me with a specific value!
      {{ end }}
    {{- end }}
  {{- end }}

  # Proxy through these input variables
  {{ range $key, $variable := (index $terraformConfigInspect "variables") }}
    {{- if not (has (index $variable "name") $.ExcludeInputVars) }}{{ index $variable "name" }} = var.{{ index $variable "name" }}
    {{ end }}
  {{- end }}
}
