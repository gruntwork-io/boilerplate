# ---------------------------------------------------------------------------------------------------------------------
# TODO: DEFINE YOUR OUTPUTS HERE
# ---------------------------------------------------------------------------------------------------------------------

output "example_output" {
  description = "example output"
  value       = "${var.example_required_input} ${var.example_optional_input}"
}