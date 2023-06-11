# AWS multi-account structure

## What does this template do?

This [boilerplate](https://github.com/gruntwork-io/boilerplate) template will configure the following multi-account
structure for AWS:

![AWS multi-account structure](https://raw.githubusercontent.com/aws-samples/aws-secure-environment-accelerator/main/src/mkdocs/docs/operations/img/ASEA-high-level-architecture.png)

It is based on the [AWS Secure Environment Accelerator](https://github.com/aws-samples/aws-secure-environment-accelerator)
architecture, but managed by our own [Landing Zone Terraform Modules](https://github.com/gruntwork-io/terraform-aws-service-catalog/tree/master/modules/landingzone).

## Features

- Create new accounts with AWS Organizations
- CloudTrail
- AWS Config
- GuardDuty
- IAM Access Analyzer
- IAM Users, Groups, Policies, Roles
- OIDC with GitHub
- Multi-region KMS CMKs
- Default EBS encryption
- These baselines meet the CIS AWS Foundations Benchmark requirements out-of-the-box.
- VPCs with route tables, subnets, NAT Gateways, Internet Gateways, and NACLs
- TGW

## Configure your deployment

This deployment needs you to provide the following inputs:

```yaml (boilerplate::input)
variables:
  - name: BillingOrg
    description: What organization does this team belong to (for billing and tagging purposes)?
    type: enum
    default: Connect
    options:
      - Connect
      - Markets
      - Tech
      - Support

  - name: TeamName
    description: Which team that will be using this AWS account?
    type: enum
    default: search-team
    options:
      - search-team
      - profile-team
      - security-team
      - data-team
      - cloud-platform-team

  - name: DefaultAwsRegion
    description: What is the default region to use for this team?
    type: enum
    default: eu-west-1
    options:
      - us-east-1
      - us-east-2
      - us-west-1
      - eu-west-1
      - ap-southeast-1
```

## Create the new AWS accounts

Here is the code to configure your new AWS account structure:

```terraform (boilerplate::template: "root/_global/account-baseline-root/main.tf")
provider "aws" {
  region = "{{ .DefaultAwsRegion }}"
}

module "account_baseline_root" {
  source = "github.com/gruntwork-io/terraform-aws-service-catalog//modules/landingzone/account-baseline-root?ref=v0.3.1"

  team_name          = "{{ .TeamName }}"
  billing_org        = "{{ .BillingOrg }}"
  default_aws_region = "{{ .DefaultAwsRegion }}"
}
```

You may also want to include some useful outputs in `outputs.tf`:

```terraform (boilerplate::template: "root/_global/account-baseline-root/outputs.tf")
output "organization_arn" {
  value = module.account_baseline_root.organization_arn
}

output "cloudtrail_trail_arn" {
  value = module.account_baseline_root.cloudtrail_trail_arn
}
```

## Commit the code and open a PR

Commit and push your code changes, open a PR, and get it reviewed and merged:

```bash (boilerplate::executable)
git add main.tf outputs.tf
git commit -m "Create new AWS multi-account structure"
git push
```

## Auto deploy

The CI / CD pipeline will automatically deploy your code, which will create your new AWS accounts.

You will be granted access via AWS SSO. 