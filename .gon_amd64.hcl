# See https://github.com/gruntwork-io/terraform-aws-ci/blob/main/modules/sign-binary-helpers/
# for further instructions on how to sign the binary + submitting for notarization.

source = ["./bin/boilerplate_darwin_amd64"]

bundle_id = "io.gruntwork.app.boilerplate"

apple_id {
  username = "machine.apple@gruntwork.io"
  password = "@env:MACOS_AC_PASSWORD"
}

sign {
  application_identity = "Developer ID Application: Gruntwork, Inc."
}

zip {
  output_path = "boilerplate_darwin_amd64.zip"
}
