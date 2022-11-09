# Terraform Module Test Template

This folder contains a boilerplate template you can use to generate a basic automated test for Terraform module. This 
test includes the common pieces you typically need, providing a faster way to create new tests, and a way to keep the 
test conventions consistent:

- Automated test written in Go and using [Terratest](https://terratest.gruntwork.io/).
- Broken down into test stages that can be run individually: `setup`, `apply`, `validate`, and `destroy`.
- Copy the code to a temp folder so there are no issues with test concurrency.
- A skeleton `README.md`.
- Set variable inputs, including generating a unique ID for namespacing.
- Run `go mod init` and `go mod tidy` to initialize Go modules (if necessary).
- Format the code using `go fmt`.

Note that this template _only_ generates an automated test. If you want to generate a module, example, and automated 
test, use the [terraform-module-full template](../terraform-module-full).

## Quick start

To use the template, install boilerplate and run:

```bash
boilerplate \
  --template-url ./blueprint \
  --output-folder "<FOLDER>" \
  --var ModuleName="<MODULE_NAME>" \
  --var RelativePathToRoot="<REL_PATH_TO_ROOT>" \
  --var RelativePathFromRootToModule="<REL_PATH_FROM_ROOT_TO_MODULE>" \
  --non-interactive
```

In the preceding command, make sure to replace the following placeholders:

- `<FOLDER>`: fill in the folder path where you want the test to be generated.
- `<NAME>`: fill in the name of the Terraform module being tested.
- `<REL_PATH_TO_ROOT>`: fill in the relative path from `<FOLDER>` to the root of the repo (the folder that contains 
  `.git`). For example, if the tests are in the `test` folder, then this value should be `..`. 
- `<REL_PATH_FROM_ROOT_TO_MODULE>`: fill in the relative path from `<REL_PATH_TO_ROOT>` to the module being tested. For
  example, if the module you're testing is in `examples/my-module`, then this value should be just that: 
  `examples/my-module`.

## Configuring the module

The boilerplate template allows certain parameters to be configured, such as the name of the module and the example
input variable to set. See the `variables` at the top of [boilerplate.yml](./blueprint/boilerplate.yml) for all 
available inputs. You can set each one with a `--var` flag, or define them in a YAML file and pass the whole file in via 
a `--var-file` flag.