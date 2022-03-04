{{- define "inputs" }}
  {{- if ne .RDSInstanceType .BaseRDSInstanceType }}
  instance_type = "{{ .RDSInstanceType }}"
  {{- end }}
  {{- if ne .RDSAllocatedStorage .BaseRDSAllocatedStorage }}
  allocated_storage = "{{ .RDSAllocatedStorage }}"
  {{- end }}

  # The DB config secret contains the following data:
  # - DB engine (e.g. postgres, mysql, etc)
  # - Default database name
  # - Port
  # - Username and password
  # Alternatively, these can be specified as individual inputs.
  db_config_secrets_manager_id = "{{ .DBSecretsManagerArn }}"
{{ end }}

{{- template "base_layout" . -}}
