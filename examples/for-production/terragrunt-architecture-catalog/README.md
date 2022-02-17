# Terragrunt Architecture Catalog Boilerplate Example

This folder contains a production level example on how to use `boilerplate` to organize a Terragrunt Architecture. The
concepts in this example is used to construct the Gruntwork Architecture Catalog, which is in turn used to generate the
[Gruntwork Reference Architecture](https://gruntwork.io/reference-architecture/).


## Quickstart

You can try running this example with the following steps:

- Make sure you have `boilerplate` version `v0.4.0` and above.

- Update the [sample_reference_architecture_vars.yml](./sample_reference_architecture_vars.yml) as necessary.

- Run `boilerplate` to invoke the `reference-architecture` blueprint with the given vars:

      boilerplate --template-url ./blueprints/reference-architecture --var-file ./sample_reference_architecture_vars.yml --output-folder ./infrastructure-live --non-interactive

You should now have a `infrastructure-live` folder that contains a full `terragrunt` example!


## How to navigate this example

The code in this example is organized into two primary folders:

1. `blueprints`: The consumable architecture blueprints for managing infrastructure components. All the Blueprints that
   you will use and invoke are defined within this folder. Refer to the [Terminology section](#terminology) below for
   more information on Blueprints.

1. `templates`: The core implementation code of this repo. All the Blueprints wrap and invoke the Templates to generate
   the infrastructure code for the underlying components into the various accounts. These are generally too low level to
   use directly, and are most useful when constructing Blueprints of your own (e.g., to create an internal Architecture
   Catalog for your users). Templates provide a useful building block to share logic across Blueprints. That is, each
   Template is potentially used by multiple Blueprints. Templates are further divided into the
   following categories:

   - `layouts`: Reusable pieces of templates that helps keep the template code DRY. These include the boilerplate
     content that are repeated across all the templates.

   - `_root`: Templates that render the root contents of a specific folder. For example, the `_root/infrastructure-live`
     template renders the content that should exist in the root folder of an `infrastructure-live` repository for
     managing live infrastructure config. This includes the root Terragrunt configuration that configures the remote
     state.

   - `services`: Templates in this directory manage a single piece of infrastructure in a larger architecture. These
     are the core building blocks of Blueprints.


## Terminology

This example is breaks down `boilerplate` templates into two categories:

* **Template**: Reusable code to generate infrastructure code to deploy and manage one piece of infrastructure in a
  single account/deployment. Since Templates only focus on a single piece of infrastructure in a single
  account/deployment, invoking a single template does not give you a full deployment for that infrastructure component.
  Instead, you need to combine the template with its dependencies, and invoke it for each account that needs the
  specific piece of infrastructure. For example, the infrastructure code for an EKS architecture is broken down into four
  templates: the VPC, the EKS control plane and worker nodes, the core Kubernetes administrative services, and the
  Kubernetes applications. Unde the hood, each template uses a single service module from the [Gruntwork Service
  Catalog](https://github.com/gruntwork-io/terraform-aws-service-catalog/). To get a full EKS architecture, you need to
  deploy all 4 templates in each environment where you wish to run EKS. E.g., You might need to deploy all 4 templates
  in dev, then use all 4 again in stage, and then use all 4 once more in prod. To make it easier to use multiple
  templates across multiple environments, the Architecture Catalog includes blueprints, as described next.

* **Blueprint**: Reusable code that combines multiple templates together to deploy a complete, self-contained piece of
  infrastructure across multiple environments. For example, the EKS blueprint will configure everything you need to
  deploy EKS into a multi-account AWS infrastructure setup. This includes the VPC, the EKS control plane and worker
  nodes, the core Kubernetes administrative services, and the Kubernetes applications, configured for each account that
  needs the infrastructure replicated. Note that Blueprints, like Templates, provide you configuration options for
  customizing the behavior of the underlying architecture. The EKS blueprint in the example above gives you
  configuration options to skip rendering the VPC template if you already have a custom VPC deployed in your existing
  architecture.

This organization exists to streamline the conditional logic of whether or not to include various components in your
architecture. Although `boilerplate` supports replicating nested folders within a template, it does not include the
ability to condition the replication. This makes it hard to implement architecture level logic, like "choose a
monitoring framework," where you want to include different modules based on selection.

This is where the **Blueprints** come in handy, where you can use `skip` directives to control which **Templates** are
invoked.
