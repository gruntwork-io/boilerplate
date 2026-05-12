// This file's name contains an unclosed template expression so the analyzer
// records a filename_render soft error for it. The body references
// ProjectName so the file still appears in the inverse index (files), giving
// the test something concrete to assert is absent from sources.
output "project" {
  value = "{{ .ProjectName }}"
}
