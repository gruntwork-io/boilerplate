#!/bin/bash

set -e

################################################################################
# Script: import-cert.sh
# Description: Imports the macOS developer certificate into the system keychain.
#              Creates a temporary keychain, imports the P12 certificate, and
#              downloads the Apple root certificate for validation. This is a
#              one-time setup step before signing binaries.
#
# Usage: import-cert.sh [--macos-skip-root-certificate]
#
# Required Environment Variables:
#   MACOS_CERTIFICATE: macOS developer certificate in P12 format (base64 encoded)
#   MACOS_CERTIFICATE_PASSWORD: macOS certificate password
#
# Optional Arguments:
#   --macos-skip-root-certificate: Skip importing Apple Root certificate
################################################################################

# Apple certificate used to validate developer certificates https://www.apple.com/certificateauthority/
readonly APPLE_ROOT_CERTIFICATE="http://certs.apple.com/devidg2.der"

function print_usage {
  echo
  echo "Usage: $0 [OPTIONS]"
  echo
  echo "Required Environment Variables:"
  echo -e "  MACOS_CERTIFICATE\t\tmacOS developer certificate in P12 format, encoded in base64."
  echo -e "  MACOS_CERTIFICATE_PASSWORD\tmacOS certificate password"
  echo
  echo "Optional Arguments:"
  echo -e "  --macos-skip-root-certificate\t\tSkip importing Apple Root certificate. Useful when running in already configured environment."
  echo -e "  --help\t\t\t\tShow this help text and exit."
}

function main {
  local mac_skip_root_certificate=""

  while [[ $# -gt 0 ]]; do
    local key="$1"
    case "$key" in
      --macos-skip-root-certificate)
        mac_skip_root_certificate=true
        shift
        ;;
      --help)
        print_usage
        exit
        ;;
      -* )
        echo "ERROR: Unrecognized argument: $key"
        print_usage
        exit 1
        ;;
      * )
        echo "ERROR: Unexpected positional argument: $1"
        print_usage
        exit 1
        ;;
    esac
  done

  ensure_macos
  import_certificate_mac "${mac_skip_root_certificate}"
}

function ensure_macos {
  if [[ $OSTYPE != 'darwin'* ]]; then
    echo -e "Certificate import is supported only on macOS"
    exit 1
  fi
}

function import_certificate_mac {
  local -r mac_skip_root_certificate="$1"
  assert_env_var_not_empty "MACOS_CERTIFICATE"
  assert_env_var_not_empty "MACOS_CERTIFICATE_PASSWORD"

  local mac_certificate_pwd="${MACOS_CERTIFICATE_PASSWORD}"
  local keystore_pw="${RANDOM}"

  # Create separated keychain file to store certificate and do quick cleanup of sensitive data
  local db_file
  db_file=$(mktemp "/tmp/XXXXXX-keychain")
  rm -rf "${db_file}"
  echo "Creating separated keychain for certificate"
  security create-keychain -p "${keystore_pw}" "${db_file}"
  security default-keychain -s "${db_file}"
  security unlock-keychain -p "${keystore_pw}" "${db_file}"
  echo "${MACOS_CERTIFICATE}" | base64 -d | security import /dev/stdin -f pkcs12 -k "${db_file}" -P "${mac_certificate_pwd}" -T /usr/bin/codesign
  if [[ "${mac_skip_root_certificate}" == "" ]]; then
    # Download Apple root certificate used as root for developer certificate
    curl -v "${APPLE_ROOT_CERTIFICATE}" --output certificate.der
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain certificate.der
  fi
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "${keystore_pw}" "${db_file}"

  echo "Certificate imported successfully"

  # NOTE: Do NOT add a trap to clean up the keychain here!
  # The keychain must persist for signing to use it.
  # Cleanup is handled after signing is complete.
}

function assert_env_var_not_empty {
  local -r var_name="$1"
  local -r var_value="${!var_name}"

  if [[ -z "$var_value" ]]; then
    echo "ERROR: Required environment variable $var_name not set."
    exit 1
  fi
}

main "$@"
