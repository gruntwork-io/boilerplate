terraform {
  required_version = "{{ .TerraformVersion }}"
}

module "{{ .ModuleName | snakecase }}" {
  source = "{{ .ModuleSource }}"

  example_required_input = "Hello"
  example_optional_input = "World"
}