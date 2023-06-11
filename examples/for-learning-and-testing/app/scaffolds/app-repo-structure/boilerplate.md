# App repo structure

## What does this template do?

This [boilerplate](https://github.com/gruntwork-io/boilerplate) template will configure a sample app and CI / CD 
pipeline:

![Sample app](https://raw.githubusercontent.com/gruntwork-io/aws-sample-app/main/_docs/sample-app-frontend-screenshot.png?token=GHSAT0AAAAAABL7OVOJSYUDXACVNSV5LJU2ZDYYX5Q)

## Features

- Package with Docker and deploy onto EKS or ECS
- Fetch secrets, such as database credentials and TLS certs, from AWS Secrets Manager
- Service discovery for talking to other microservices
- Talk to databases and caches over TLS
- Apply schema migrations to a database
- Define and manage configurations as code for multiple environments (dev, stage, prod, etc)
- Automated tests
- CI / CD pipeline

## Configure your app

This deployment needs you to provide the following inputs:

```yaml (boilerplate::input)
variables:
  - name: TeamName
    description: Which team owns this app?
    type: enum
    default: search-team
    options:
      - search-team
      - profile-team
      - security-team
      - data-team
      - cloud-platform-team

  - name: LanguageAndFramework
    description: Which supported language / framework do you wish to use for this app?
    type: enum
    default: Java/Spring
    options:
      - Java/Spring
      - JavaScript/Express.js
      - TypeScript/Next.js
      - Ruby/Rails

  - name: IncludeSchemaMigrations
    description: Should we include schema migrations for a relational DB?
    type: bool
    default: true

  - name: IncludeSecretsExample
    description: Should we include an example of how to fetch secrets from AWS Secrets Manager?
    type: bool
    default: true
```

## Create the new AWS accounts

Here is the code to configure your new AWS account structure:

```terraform (boilerplate::template: "root/_global/account-baseline-root/main.tf")
provider "github" {
}

module "sample_app" {
  source = "github.com/gruntwork-io/terraform-aws-service-catalog//modules/landingzone/sample-app?ref=v0.4.2"

  team_name                 = "{{ .TeamName }}"
  lang_and_framework        = "{{ .LanguageAndFramework }}"
  include_schema_migrations = {{ .IncludeSchemaMigrations }}
  include_secrets_example   = {{ .IncludeSecretsExample }}
}
```

You may also want to include some useful outputs in `outputs.tf`:

```terraform (boilerplate::template: "root/_global/account-baseline-root/outputs.tf")
output "repo_name" {
  value = module.account_baseline_root.repo_name
}
```

## Commit the code and open a PR

Commit and push your code changes, open a PR, and get it reviewed and merged:

```bash (boilerplate::executable)
git add main.tf outputs.tf
git commit -m "Create new application"
git push
```

## Auto deploy

The CI / CD pipeline will automatically deploy your code, which will create your repos, and configure your CI / CD 
pipeline.
