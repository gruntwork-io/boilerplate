{{- $moduleInspectParsed := .UnderlyingModuleInspectJsonOutput | mustFromJson -}}
{{- $terraformConfigInspect := index $moduleInspectParsed "inspect-output" -}}

#---------------------------------------------------------------------------------------------------------------------
# CREATE THE WRAPPER MODULE
# ---------------------------------------------------------------------------------------------------------------------

module "{{ .WrapperModuleName | snakecase }}" {
  source = "{{ .UnderlyingModuleSourceUrl }}"

  # Proxy through input variables
  {{- range $key, $variable := (index $terraformConfigInspect "variables") }}
    {{ index $variable "name" }} = var.{{ index $variable "name" }}
  {{- end }}
}
