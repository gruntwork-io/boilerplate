terraform {
  required_version = "{{ .TofuVersion }}"
}

module "{{ .ModuleName | snakecase }}" {
  source = "{{ .ModuleSource }}"

  example_required_input = "Hello"
  example_optional_input = "World"
}