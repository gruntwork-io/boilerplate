terraform {
  required_version = "1.6.2"
}

module "vpc" {
  source = "../../modules/vpc"

  example_required_input = "Hello"
  example_optional_input = "World"
}