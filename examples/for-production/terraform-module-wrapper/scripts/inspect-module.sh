#!/usr/bin/env bash

set -e

# Parse CLI args
if [[ "$#" -ne 2 ]]; then
  >&2 echo "Usage: inspect-module.sh UNDERLYING_MODULE_SOURCE_url OUTPUT_FOLDER"
  exit 1
fi

underlying_module_source_url="$1"
output_folder="$2"

# Check all required binaries are installed
required_binaries=("terraform" "terraform-config-inspect" "readlink" "hcledit")
for binary in "${required_binaries[@]}"; do
  if ! command -v "$binary" &> /dev/null; then
    >&2 echo "Required binary '$binary' not found. Is it installed and in PATH?"
    exit 1
  fi
done

if [[ "$underlying_module_source_url"  == ".."* ]]; then
  # Turn relative file paths into absolute file paths, as we're running 'terraform init' in a temp folder, so the
  # relative file path wouldn't work in there. Note also that 'terraform init' struggles with symlinks, so we use
  # readlink to resolve those symlink. Finally, we create the output folder if it doesn't exist already, or the
  # relative path won't work.
  mkdir -p "$output_folder"
  underlying_module_source_url=$(readlink -f "$output_folder/$underlying_module_source_url")
fi

if [[ "$underlying_module_source_url"  == "/"* ]]; then
  # Force go-getter to recognize file paths as file paths
  underlying_module_source_url="file::$underlying_module_source_url"
fi

# Check out the underlying module into a temp dir
tmp_dir=$(mktemp -d -t terraform-module-wrapper)
cd "$tmp_dir"
terraform init \
  -input=false \
  -get=false \
  -backend=false \
  -from-module="$underlying_module_source_url" 1>&2

# Now run terraform-config-inspect, which will write the JSON to stdout
inspect_json=$(terraform-config-inspect --json)

# Write the results as JSON to stdout so it can be read in and parsed by boilerplate templates
echo -n "{\"inspect-output\": $inspect_json, \"underlying-module-path\": \"$tmp_dir\"}"

