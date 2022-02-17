{{- define "inputs" }}
  vpc_name              = "{{ .AppVPCName }}"
  num_nat_gateways      = {{ .AppVPCNumNATGateways }}
{{- if gt .AppVPCNumAvailabilityZones 0 }}
  num_availability_zones = {{ .AppVPCNumAvailabilityZones }}
{{- end }}

  # To simplify the example, this project deploys VPCs without flow logs.
  create_flow_logs = false
{{ end }}

{{- template "base_envcommon_layout" . -}}
