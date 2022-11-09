# Tests

This folder contains the tests for the modules in this repo.

## Running the tests locally

**Note #1**: Many of these tests create real resources in an AWS account. That means they cost money to run, especially
if you don't clean up after yourself. Please be considerate of the resources you create and take extra care to clean
everything up when you're done!

**Note #2**: Never hit `CTRL + C` or cancel a build once tests are running or the cleanup tasks won't run!

**Note #3**: Set a high time out (e.g., `-timeout 60m`) on all tests, not because they necessarily take that long, but 
because Go has a default test timeout of 10 minutes, after which it does a `SIGQUIT`, preventing the tests from properly 
cleaning up after themselves. Therefore, you should set a high timeout to make sure all tests have enough time to finish 
and cleanup.

### Prerequisites

- Install the latest version of [Go](https://golang.org/).
- Install [Terraform](https://www.terraform.io/downloads.html).
- Add your AWS credentials as environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- For some of the tests, you also need to set the `GITHUB_OAUTH_TOKEN` environment variable to a valid GitHub
  auth token with "repo" access. You can generate one here: https://github.com/settings/tokens

### Run all the tests

```bash
cd test
go test -v -timeout 60m
```

### Run a specific test

To run a specific test called `TestFoo`:

```bash
cd test
go test -v -timeout 60m -run TestFoo
```