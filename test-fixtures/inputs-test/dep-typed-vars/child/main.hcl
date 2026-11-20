regions = [{{ range $index, $region := .Regions }}{{ if $index }}, {{ end }}"{{ $region }}"{{ end }}]
budget  = {{ add .Budget 1 }}
ratio   = {{ printf "%.2f" .Ratio }}
enabled = {{ if .Enabled }}"yes"{{ else }}"no"{{ end }}
{{- range $key, $value := .Tags }}
tag_{{ $key }} = "{{ $value }}"
{{- end }}
