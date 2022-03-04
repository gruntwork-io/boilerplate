{{- define "inputs" }}
  {{- if ne .RedisInstanceType .BaseRedisInstanceType }}
  instance_type = "{{ .RedisInstanceType }}"
  {{- end }}
{{ end }}

{{- template "base_layout" . -}}
