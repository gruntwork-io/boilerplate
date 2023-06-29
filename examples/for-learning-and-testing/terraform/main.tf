provider "aws" {
  region = "us-east-1"
}

resource "aws_instance" "example" {
  ami           = "ami-abcd1234"
  instance_type = "t3.micro"

  tags = {
    Name = "{{ .ServerName }}"
  }
}