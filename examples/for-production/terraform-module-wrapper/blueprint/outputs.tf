{{- $moduleInspectParsed := .UnderlyingModuleInspectJsonOutput | mustFromJson -}}
{{- $terraformConfigInspect := index $moduleInspectParsed "inspect-output" -}}

#---------------------------------------------------------------------------------------------------------------------
# PROXY THROUGH OUTPUT VARIABLES
# ---------------------------------------------------------------------------------------------------------------------

{{- range $key, $variable := (index $terraformConfigInspect "outputs") }}
  {{- if not (has (index $variable "name") $.ExcludeOutputVars) }}

    output "{{ index $variable "name" }}" {
      value = module.{{ $.WrapperModuleName | snakecase }}.{{ index $variable "name" }}
    }
  {{- end }}
{{- end }}