terraform {
    source = "github.com/gruntwork-io/terraform-aws-service-catalog//modules/networking/vpc?ref=v0.3.1"
}

inputs = {
    aws_region       = "{{ .AwsRegion }}"
    
    vpc_name         = "{{ .VpcName }}"
    cidr_block       = "{{ .CidrBlock }}"
    num_nat_gateways = {{ .NumNatGateways }}
}