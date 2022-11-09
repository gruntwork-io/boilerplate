#---------------------------------------------------------------------------------------------------------------------
# CREATE AN EXAMPLE RESOURCE
# ---------------------------------------------------------------------------------------------------------------------

resource "random_pet" "example" {
  keepers = {
    required = var.example_required_input_var
    optional = var.example_optional_input_var
  }

  length = 2
  prefix = var.example_required_input_var
}