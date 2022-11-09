# Terraform Module Wrapper Template

This folder contains a boilerplate template you can use to generate a wrapper of an underlying Terraform module, which 
is useful for extending the module or customizing its behavior (e.g., we create lots of wrapper modules in our CIS 
Service Catalog). This wrapper module includes the common pieces you typically need, providing a faster way to create 
new wrapper modules, and a way to keep the module conventions consistent:

- Create the `module` block and pass through all input variables.
- Create proxy input variables for all input variables in the underlying module (other than those in an exclude list).
- Create proxy output variables for all output variables in the underyling module (other than those in an exclude list).
- Set the Terraform `required_version`.
- Create a `required_providers` block.
- Create a skeleton README.
- Our standard commenting convention.
- Format the code using `terraform fmt`.

## Quick start

To use the template, you must first install the following tools and ensure they are in your `PATH`: 

1. Boilerplate: from this repo.
2. [Terraform](https://terraform.io/)
3. [terraform-config-inspect](https://github.com/hashicorp/terraform-config-inspect)
4. [hcledit](https://github.com/minamijoyo/hcledit)
5. `readlink`: Probably already installed. But if not, try `brew install coreutils`.

Once these are all installed, run the following:

```bash
boilerplate \
  --template-url ./blueprint \
  --output-folder "<FOLDER>" \
  --var-file "<VAR_FILE>"
  --non-interactive
```

In the preceding command, make sure to replace the following placeholders:

- `<FOLDER>`: fill in the folder path where you want the wrapper module to be generated.
- `<VAR_FILE>`: fill in the path to a YAML variable file. We recommend using a file so you can check it into version 
  control along with the generated wrapper module, so that you can re-run this process in the future to update the 
  wrapper module to the latest. The variable file must set at least the following input variables:
    - `<WRAPPER_MODULE_NAME>`: fill in the name to use for the generated wrapper module.
    - `<UNDERLYING_MODULE_SRC_URL>`: fill in the `source` URL to use for the underlying module. This can be any [source URL
      supported by Terraform](https://developer.hashicorp.com/terraform/language/modules/sources), including local file 
      paths, Git/SSH URLs, Git/HTTPS URLs, etc.

For example, to create a wrapper module of the [vpc-app 
module](https://github.com/gruntwork-io/terraform-aws-vpc/tree/main/modules/vpc-app), you can run the following:

```bash
boilerplate \
  --template-url ./blueprint \
  --output-folder "<FOLDER>" \
  --var-file example_wrapper_vars.yml
  --non-interactive
```

This uses the example var file [example_wrapper_vars.yml](./example_wrapper_vars.yml), which does the following:

- Creates a wrapper module also called `vpc-app`
- Sets the `source` URL to a Git/SSH URL for `vpc-app`, at a specific version.
- Excludes several input variables from being proxied, as we will set these variables to custom values in the wrapper 
  module.

## Configuring the module

The boilerplate template allows certain parameters to be configured, including, most usefully, the input and output 
variables that should _not_ be proxied (e.g., because your wrapper module does not want to expose these variables), 
which you can configure using the `ExcludeInputVars` and `ExcludeOutputVars` variables. See the `variables` at the top 
of [boilerplate.yml](./blueprint/boilerplate.yml) for all available inputs. You can set each one with a `--var` flag, 
or define them in a YAML file and pass the whole file in via a `--var-file` flag (recommended). See
[example_wrapper_vars.yml](./example_wrapper_vars.yml) for an example variable file.