{{- define "locals" }}
  {{- if gt (len .AppVPCCIDRBlock) 0 }}
  cidr_block = "{{ .AppVPCCIDRBlock }}"
  {{- end }}
{{- end }}

{{- define "inputs" }}
  {{- if gt (len .AppVPCCIDRBlock) 0 }}
  cidr_block = local.cidr_block
  {{- end }}
{{- end }}

{{- template "base_layout" . -}}
