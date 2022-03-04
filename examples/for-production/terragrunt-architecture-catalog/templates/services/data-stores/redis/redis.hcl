{{- define "dependencies" }}
dependency "vpc" {
  config_path = "{{ .AppVPCPath }}"

  mock_outputs = {
    vpc_id                 = "{{ .MockVPCID }}"
    private_persistence_subnet_ids = [ {{ range $subnet := .MockVPCSubnets }}"{{ $subnet }}", {{ end }} ]
    private_app_subnet_cidr_blocks = [ {{ range $cidr := .MockVPCSubnetCIDRBlocks }}"{{ $cidr }}", {{ end }} ]
  }
  mock_outputs_allowed_terraform_commands = [{{ range $cmd := .AllowedMockCommands }}"{{ $cmd }}", {{ end }}]
}
{{- end }}

{{- define "inputs" }}
  name = "{{ .RedisClusterName }}"
  instance_type = "{{ .BaseRedisInstanceType }}"
  vpc_id = dependency.vpc.outputs.vpc_id
  subnet_ids = dependency.vpc.outputs.private_persistence_subnet_ids
  redis_version = "{{ .RedisVersion }}"

  replication_group_size = {{ .RedisReplicationGroupSize }}
  enable_multi_az = false
  enable_automatic_failover = false
  parameter_group_name = "{{ .RedisParameterGroupName }}"
  enable_cloudwatch_alarms = true

  # Here we allow any connection from the private app subnet tier of the VPC. You can further restrict network access by
  # security groups for better defense in depth.
  allow_connections_from_cidr_blocks     = dependency.vpc.outputs.private_app_subnet_cidr_blocks

  # Only apply changes during the scheduled maintenance window, as certain DB changes cause degraded performance or
  # downtime. For more info, see: https://docs.aws.amazon.com/AmazonElastiCache/latest/mem-ug/Clusters.Modify.html
  # We default to false, but in non-prod environments we set it to true to immediately roll out the changes.
  apply_immediately = false
{{ end }}

{{- template "base_envcommon_layout" . -}}
