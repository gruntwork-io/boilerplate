terraform {
  #   source = "git::ssh://git@github.com/gruntwork-io/terraform-aws-eks.git//modules/eks-cluster-control-plane?ref=v0.56.0"
  # There is an intentional typo in the module name here so we can test error handling in 'patcher update'
  source = "git::ssh://git@github.com/gruntwork-io/terraform-aws-eks.git//modules/eks-control-plane?ref=v0.55.0"
}
