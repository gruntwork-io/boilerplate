# Terraform Module Full Template

This folder contains a boilerplate template you can use to generate a full Terraform module, including the module, 
example, and test code. This test includes the common pieces you typically need, providing a faster way to create new 
modules, and a way to keep the module conventions consistent:

- `modules` folder with the module itself, including all the features from the [terraform-module 
  template](../terraform-module): standard file naming convention, example resources, standard commenting convention, 
  etc.
- `examples` folder with an example usage of the module, generated using the [terraform-module-wrapper 
  template](../terraform-module-wrapper), including setting all input variables, proxying through output variables, etc. 
- `test` folder with an automated test for the example usage, generated using the [terraform-wrapper-test 
  template](../terraform-module-test), including the Go test, pointed at the example usage, with test stages, unique IDs
  for namespacing, Go module initialization, etc.

## Quick start

To use the template, install boilerplate and run:

```bash
boilerplate \
  --template-url ./blueprint \
  --output-folder "<FOLDER>" \
  --var ModuleName="<MODULE_NAME>" \
  --non-interactive
```

In the preceding command, make sure to replace the following placeholders:

- `<FOLDER>`: fill in the folder path where you want the module, examples, and test to be generated.
- `<NAME>`: fill in the name of the Terraform module to create.

## Configuring the module

The boilerplate template allows certain parameters to be configured, such as the name of the module and the Terraform 
version to be set. See the `variables` at the top of [boilerplate.yml](./blueprint/boilerplate.yml) for all 
available inputs. You can set each one with a `--var` flag, or define them in a YAML file and pass the whole file in via 
a `--var-file` flag.