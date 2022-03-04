# Blueprints

This folder contains boilerplate blueprints as defined in the [Terminology
section](/examples/for-production/terragrunt-architecture-catalog/README.md#terminology) of the root README.

Each blueprint provides a standalone template that configures the entire `infrastructure-live` repository with all the
necessary infrastructure components that are required to deploy it. For example, the `vpc-app` blueprint not only
includes the templates for setting up the App VPC in the specific account, but it also sets up the envcommon folder with
the common configurations for any App VPC.

Note that some of the blueprints may call other blueprints to setup what the dependencies. For example, the `postgres`
blueprint will call the `vpc-app` blueprint to ensure an App VPC exists in the architecture to house the RDS database.

This allows you to compose the blueprints together into a full scale customized architecture, as is done in the
`reference-architecture` blueprint:

- The `reference-architecture` blueprint sets up the folder structure of an `infrastructure-live` repository.
- It then uses the `reference-architecture-app-account` blueprint to set up three copies of the infrastructure in a
  `dev`, `stage`, and `prod` account.
- The `reference-architecture-app-account` blueprint in turn calls the `vpc-app`, `postgres`, and `redis` blueprints to
  setup those components within the scope of a single account.
