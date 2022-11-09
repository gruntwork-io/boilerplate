# Terraform Module Template

This folder contains a boilerplate template you can use to generate a basic Terraform module. This module includes the
common pieces you typically need, providing a faster way to create a new modules, and a way to keep the module 
conventions consistent:

- Standard file naming convention: `variables.tf`, `outputs.tf`, `main.tf`, `dependencies.tf`, `versions.tf`.
- A skeleton `README.md`.
- Some example resources, input variables, and output variables.
- Our standard commenting convention.
- Format the code using `terraform fmt`.

Note that this template _only_ generates a module. If you want to generate a module, example, and automated test, use
the [terraform-module-full template](../terraform-module-full).

## Quick start

To use the template, install boilerplate and run:

```bash
boilerplate \
  --template-url ./blueprint \
  --output-folder "<FOLDER>" \
  --var ModuleName="<NAME>" \
  --non-interactive
```

In the preceding command, make sure to replace the following placeholders:

- `<FOLDER>`: fill in the folder path where you want the module to be generated.
- `<NAME>`: fill in the name you want to use for the module.

## Configuring the module

The boilerplate template allows certain parameters to be configured, such as the name of the module and the Terraform 
version. See the `variables` at the top of [boilerplate.yml](./blueprint/boilerplate.yml) for all available inputs.
You can set each one with a `--var` flag, or define them in a YAML file and pass the whole file in via a `--var-file`
flag.