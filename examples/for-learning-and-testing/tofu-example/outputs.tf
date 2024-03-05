output "example_output" {
  description = "example output"
  value       = module.{{ .ModuleName | snakecase }}.example_output
}