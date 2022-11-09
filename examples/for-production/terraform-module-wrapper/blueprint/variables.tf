{{- $moduleInspectParsed := .UnderlyingModuleInspectJsonOutput | mustFromJson -}}
{{- $terraformConfigInspect := index $moduleInspectParsed "inspect-output" -}}
{{- $underlyingModulePath := index $moduleInspectParsed "underlying-module-path" -}}

{{- define "variable" -}}
{{ shell "hcledit" "block" "get" "-f" (printf "%s/%s" .underlyingModulePath (index (index .variable "pos") "filename")) (printf "variable.%s" (index .variable "name")) }}
{{ end -}}
#---------------------------------------------------------------------------------------------------------------------
# REQUIRED MODULE PARAMETERS
# These variables must be passed in by the operator.
# ---------------------------------------------------------------------------------------------------------------------

{{ range $key, $variable := (index $terraformConfigInspect "variables") }}
  {{- if and (not (index $variable "default")) (not (has (index $variable "name") $.ExcludeInputVars)) }}{{ template "variable" (dict "variable" $variable "underlyingModulePath" $underlyingModulePath) }}{{ end -}}
{{ end -}}

# ---------------------------------------------------------------------------------------------------------------------
# OPTIONAL MODULE PARAMETERS
# These variables have defaults, but may be overridden by the operator.
# ---------------------------------------------------------------------------------------------------------------------

{{ range $key, $variable := (index $terraformConfigInspect "variables") }}
  {{- if and (index $variable "default") (not (has (index $variable "name") $.ExcludeInputVars)) }}{{ template "variable" (dict "variable" $variable "underlyingModulePath" $underlyingModulePath) }}{{ end -}}
{{ end }}


