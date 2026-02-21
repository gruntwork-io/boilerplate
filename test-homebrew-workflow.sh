#!/usr/bin/env bash
#
# Test Homebrew formula generation locally without pushing anything.
# Run from the root of your tool's repo.
#
# SYNC NOTE: The formula generation logic in this script is intentionally
# duplicated from update-homebrew.yml so it can run standalone.
# If you change formula generation here, update the workflow too
# (and vice versa). See "Keeping the test script in sync" in the
# homebrew-tap maintainers guide:
# https://github.com/gruntwork-io/homebrew-tap/tree/main/maintainers
#
# Usage:
#   ./test-homebrew-workflow.sh <release-tag> [version-cmd]
#
# Arguments:
#   release-tag   The GitHub release tag (e.g., v1.0.0, beta-v0.5.2)
#   version-cmd   The flag your tool uses to print its version (default: --version)
#
# Examples:
#   ./test-homebrew-workflow.sh v1.0.0
#   ./test-homebrew-workflow.sh beta-v0.5.2 version
#
# Requirements:
#   - gh CLI installed and authenticated (https://cli.github.com/)
#   - Run from the root of a git repo hosted on github.com/gruntwork-io/<tool>

set -euo pipefail

TAG="${1:?Usage: $0 <release-tag> [version-cmd]}"
VERSION_CMD="${2:---version}"

TOOL_NAME=$(basename "$(git rev-parse --show-toplevel)")
REPO="gruntwork-io/${TOOL_NAME}"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Homebrew Formula Test — ${TOOL_NAME} ${TAG}"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# ── Platform-portable sha256 ──────────────────────────────────────
sha256_hash() {
  if command -v sha256sum &>/dev/null; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

# ── Download binaries ─────────────────────────────────────────────
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

echo "==> Downloading release binaries from ${REPO} @ ${TAG}..."
PLATFORMS=(darwin_arm64 darwin_amd64 linux_arm64 linux_amd64)
for platform in "${PLATFORMS[@]}"; do
  echo "    ${TOOL_NAME}_${platform}"
  gh release download "$TAG" \
    --repo "$REPO" \
    --pattern "${TOOL_NAME}_${platform}" \
    --dir "$WORK/binaries"
done
echo ""

# ── Compute checksums ─────────────────────────────────────────────
echo "==> SHA256 checksums:"
for platform in "${PLATFORMS[@]}"; do
  sha=$(sha256_hash "$WORK/binaries/${TOOL_NAME}_${platform}")
  # Store each checksum in a plain variable (avoids bash 4+ associative arrays).
  eval "SHA_${platform}=${sha}"
  printf "    %-20s %s\n" "${platform}" "$sha"
done
echo ""

# ── Derive names ──────────────────────────────────────────────────
CLASS_NAME=$(echo "$TOOL_NAME" | perl -pe 's/(^|[-_])(\w)/uc($2)/ge')
SEMVER=$(echo "$TAG" | sed -E 's/^[a-zA-Z]*-?v?//' | sed -E 's/-.*//')
VERSIONED_CLASS="${CLASS_NAME}AT$(echo "$SEMVER" | tr -d '.')"

echo "==> Derived names:"
echo "    Tool name:        ${TOOL_NAME}"
echo "    Class name:       ${CLASS_NAME}"
echo "    Semver:           ${SEMVER}"
echo "    Versioned class:  ${VERSIONED_CLASS}"
echo "    Version cmd:      ${TOOL_NAME} ${VERSION_CMD}"
echo ""

# ── Generate formula helper ───────────────────────────────────────
generate_formula() {
  local class_name="$1"
  local desc="${2:-CLI tool}"
  local homepage="${3:-https://github.com/${REPO}}"
  local license="${4:-MPL-2.0}"

  cat <<EOF
class ${class_name} < Formula
  desc "${desc}"
  homepage "${homepage}"
  version "${TAG}"
  license "${license}"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/${REPO}/releases/download/${TAG}/${TOOL_NAME}_darwin_arm64"
      sha256 "${SHA_darwin_arm64}"
    else
      url "https://github.com/${REPO}/releases/download/${TAG}/${TOOL_NAME}_darwin_amd64"
      sha256 "${SHA_darwin_amd64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/${REPO}/releases/download/${TAG}/${TOOL_NAME}_linux_arm64"
      sha256 "${SHA_linux_arm64}"
    else
      url "https://github.com/${REPO}/releases/download/${TAG}/${TOOL_NAME}_linux_amd64"
      sha256 "${SHA_linux_amd64}"
    end
  end

  def install
    binary = Dir["${TOOL_NAME}_*"].first
    bin.install binary => "${TOOL_NAME}"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/${TOOL_NAME} ${VERSION_CMD}")
  end
end
EOF
}

# ── Try to read metadata from an existing formula in homebrew-tap ──
DESC="<REPLACE with your tool description>"
HOMEPAGE="https://github.com/${REPO}"
LICENSE="MPL-2.0"

HOMEBREW_TAP_DIR=""
if [[ -d "../homebrew-tap/Formula" ]]; then
  HOMEBREW_TAP_DIR="../homebrew-tap"
elif [[ -n "${HOMEBREW_TAP_PATH:-}" && -d "${HOMEBREW_TAP_PATH}/Formula" ]]; then
  HOMEBREW_TAP_DIR="$HOMEBREW_TAP_PATH"
fi

if [[ -n "$HOMEBREW_TAP_DIR" && -f "${HOMEBREW_TAP_DIR}/Formula/${TOOL_NAME}.rb" ]]; then
  echo "==> Reading metadata from ${HOMEBREW_TAP_DIR}/Formula/${TOOL_NAME}.rb"
  EXISTING="${HOMEBREW_TAP_DIR}/Formula/${TOOL_NAME}.rb"
  DESC=$(grep '^\s*desc ' "$EXISTING" | head -1 | sed 's/.*desc "\(.*\)"/\1/')
  HOMEPAGE=$(grep '^\s*homepage ' "$EXISTING" | head -1 | sed 's/.*homepage "\(.*\)"/\1/')
  LICENSE=$(grep '^\s*license ' "$EXISTING" | head -1 | sed 's/.*license "\(.*\)"/\1/')
  echo ""
fi

# ── Write formula files ──────────────────────────────────────────
OUTDIR="$WORK/Formula"
mkdir -p "$OUTDIR"

UNVERSIONED="$OUTDIR/${TOOL_NAME}.rb"
VERSIONED="$OUTDIR/${TOOL_NAME}@${SEMVER}.rb"

generate_formula "$CLASS_NAME" "$DESC" "$HOMEPAGE" "$LICENSE" > "$UNVERSIONED"
generate_formula "$VERSIONED_CLASS" "$DESC" "$HOMEPAGE" "$LICENSE" > "$VERSIONED"

# ── Print results ─────────────────────────────────────────────────
echo "==> Generated: ${TOOL_NAME}.rb"
echo "────────────────────────────────────────────────────────────────"
cat "$UNVERSIONED"
echo ""
echo "==> Generated: ${TOOL_NAME}@${SEMVER}.rb"
echo "────────────────────────────────────────────────────────────────"
cat "$VERSIONED"
echo ""

echo "==> Files written to: ${OUTDIR}/"
ls -la "$OUTDIR/"
echo ""
echo "Done. Review the formulae above. No changes were pushed."
