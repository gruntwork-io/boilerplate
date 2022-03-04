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
  name = "{{ .RDSDatabaseName }}"
  engine = "{{ .RDSEngine }}"
  engine_version = "{{ .RDSEngineVersion }}"
  instance_type = "{{ .BaseRDSInstanceType }}"
  allocated_storage = "{{ .BaseRDSAllocatedStorage }}"

  # Configure the logs that should be shipped to CloudWatch
  enabled_cloudwatch_logs_exports = ["error", "slowquery"]

  # Enable deletion protection
  # Deletion protection is a precaution to avoid accidental data loss by protecting the instance from being deleted.
  enable_deletion_protection = true

  # Specifies whether to deploy a standby instance to another availability zone. RDS will automatically failover
  # to the standby in the event of a problem with the primary.
  multi_az = false

  # We deploy RDS inside the private persistence tier.
  vpc_id = dependency.vpc.outputs.vpc_id
  subnet_ids = dependency.vpc.outputs.private_persistence_subnet_ids

  # Here we allow any connection from the private app subnet tier of the VPC. You can further restrict network access by
  # security groups for better defense in depth.
  allow_connections_from_cidr_blocks     = dependency.vpc.outputs.private_app_subnet_cidr_blocks

  # The RDS service module will create a KMS CMK. With these settings, we allow the key access to be managed via IAM.
  # To understand how this works, see: https://docs.aws.amazon.com/kms/latest/developerguide/key-policies.html
  cmk_administrator_iam_arns = [
    "arn:aws:iam::${local.account_id}:root",
  ]
  cmk_user_iam_arns = [
    {
      name       = ["arn:aws:iam::${local.account_id}:root"],
      conditions = [],
    },
  ]

  # Only apply changes during the scheduled maintenance window, as certain DB changes cause degraded performance or
  # downtime. For more info, see:
  # http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Overview.DBInstance.Modifying.html
  # Set this to true to immediately roll out the changes.
  apply_immediately = false
{{ end }}

{{- template "base_envcommon_layout" . -}}
