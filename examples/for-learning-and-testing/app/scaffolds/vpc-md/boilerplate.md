# Deploy a Production-grade VPC

## What is a VPC?

If you were building your own data center, you would set up a physical network by
configuring switches, routers, connecting everything with ethernet cables, and so
on. In the AWS cloud, you can set up a virtual network (AKA a software-defined
network) by configuring a Virtual Private Cloud (VPC). Each VPC defines a logical
partition of your AWS account, complete with its own IP addresses, subnets,
routing rules, and firewalls.

The VPC serves three primary purposes:

* **Networking**:
  The VPC is the basic networking and communication layer for your AWS account.
  Just about every AWS resource (e.g., EC2 instances, RDS databases, ELBs, etc)
  runs in a VPC and the VPC determines how (or if) all those resources are able
  to talk to each other.

* **Security**:
  The VPC is also the basic security layer for your AWS account. As it controls
  all networking and communication, it’s your first line of defense against
  attackers, protecting your resources from unwanted access.

* **Partitioning**:
  VPCs also give you a way to create separate, logical partitions within an AWS
  account. For example, you could have one VPC for a staging environment and a
  separate VPC for a production environment. You can also connect VPCs to other
  networks, such as connecting your VPC to your corporate intranet via a VPN
  connection, so that everyone in your office and all the resources in your AWS
  account can access the same IPs and domain names.

## Configure your deployment

This deployment needs you to provide the following inputs:

```yaml (boilerplate::input)
variables:
  - name: AwsRegion
    description: Into which region should the VPC be deployed?
    type: enum
    default: eu-west-1
    options:
      - us-east-1
      - us-east-2
      - us-west-1
      - eu-west-1
      - ap-southeast-1

  - name: VpcName
    description: What should the VPC be called?
    type: string
    default: my-vpc

  - name: CidrBlock
    description: What should the CIDR block be for the VPC?
    type: string
    default: 10.0.0.0/16

  - name: NumNatGateways
    description: How many NAT Gateways should be deployed in the VPC?
    type: int
    default: 1  
```

## Generate the module

To deploy given the inputs above, you can use the following code in `main.tf`:

```terraform (boilerplate::template: "examples/vpc/main.tf")
provider "aws" {
  region = "{{ .AwsRegion }}"
}

module "vpc" {
  source = "github.com/gruntwork-io/terraform-aws-service-catalog//modules/networking/vpc?ref=v0.3.1"

  vpc_name         = "{{ .VpcName }}"
  cidr_block       = "{{ .CidrBlock }}"
  num_nat_gateways = {{ .NumNatGateways }}
}
```

You may also want to include some useful outputs in `outputs.tf`:

```terraform (boilerplate::template: "examples/vpc/outputs.tf")
output "vpc_id" {
  value = module.vpc.vpc_id
}

output "vpc_name" {
  value = module.vpc.vpc_name
}

output "vpc_cidr_block" {
  value = module.vpc.vpc_cidr_block
}
```

## Generate the tests

And here's an automated test for the generated module:

```go (boilerplate::template: "test/vpc_test.go")
package test

import (
	"testing"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/require"
)

func TestVpc(t *testing.T) {
	t.Parallel()

	terraformOptions := &terraform.Options{
		TerraformDir: "../examples/vpc",
	}

	defer terraform.Destroy(t, terraformOptions)
	terraform.InitAndApply(t, terraformOptions)

	vpc_id := terraform.Output(t, terraformOptions, "vpc_id")
	require.Regexp(t, "vpc-.+", vpc_id)

	vpc_name := terraform.Output(t, terraformOptions, "vpc_name")
	require.Equal(t, "{{ .VpcName }}", vpc_name)

	vpc_cidr_block := terraform.Output(t, terraformOptions, "vpc_cidr_block")
	require.Equal(t, "{{ .CidrBlock }}", vpc_cidr_block)	
}
```

You'll also need this `go.mod` file to configure the dependencies:

```go (boilerplate::template: "test/go.mod")
module test

go 1.18

require (
    github.com/gruntwork-io/terratest v0.40.12
)
```

## Run the test

Run the test to make sure the module works as expected:

1. [Install Go](https://go.dev/doc/install) (minimum version: 1.18).
2. Authenticate to a test AWS account.
3. Run the tests:

```bash (boilerplate::executable)
cd test
go test -v -timeout 60m
```

## Deploy the module

Now that your deployment is tested, you can deploy by running:

1. Install [Terragrunt](https://terragrunt.gruntwork.io/).
2. Authenticate to a real AWS account.
3. Deploy:

```bash (boilerplate::executable)
cd examples/vpc
terragrunt apply
```