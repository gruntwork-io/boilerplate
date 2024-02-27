terraform {
  required_version = "{{ .TerraformVersion }}"

  {{- if .Providers }}

  required_providers {
    {{ range $provider_source, $provider_version := .Providers -}}
    {{ $provider_source }} = {
      source  = "hashicorp/{{ $provider_source }}"
      version = "{{ $provider_version }}"
    }
    {{- end }}
  }
  {{- end }}
}

# ---------------------------------------------------------------------------------------------------------------------
# TODO: DEFINE YOUR RESOURCES HERE
# ---------------------------------------------------------------------------------------------------------------------
