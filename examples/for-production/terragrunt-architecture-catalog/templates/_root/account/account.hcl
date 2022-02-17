# Set account-wide variables
locals {
  account_name = "{{ .AWSAccountName }}"
  account_id   = "{{ .AWSAccountID }}"
}
