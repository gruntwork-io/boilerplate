terraform {
  source = "."
}

inputs = {
  region = "us-west-2"
  vpc_cidr = "10.0.0.1/"
}