#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: $0 <tag> <content-file> <assets-dir> <output-file>" >&2
  exit 2
fi

tag="$1"
content_file="$2"
assets_dir="$3"
output_file="$4"

if [[ ! -f "${content_file}" ]]; then
  echo "release content file not found: ${content_file}" >&2
  exit 1
fi

{
  echo "# NetGoat Agent ${tag}"
  echo
  sed '1{/^# /d;}' "${content_file}"
  echo
  echo "## Release Assets"
  echo
  echo "| Platform | Archive | SHA-256 |"
  echo "| --- | --- | --- |"

  find "${assets_dir}" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) | sort | while read -r asset; do
    file="$(basename "${asset}")"
    platform="${file#netgoat-agent-}"
    platform="${platform%.tar.gz}"
    platform="${platform%.zip}"
    checksum_file="${asset}.sha256"
    checksum=""
    if [[ -f "${checksum_file}" ]]; then
      checksum="$(awk '{print $1}' "${checksum_file}")"
    fi
    echo "| ${platform} | \`${file}\` | \`${checksum}\` |"
  done

  echo
  echo "## Build Provenance"
  echo
  echo "- Commit: \`${GITHUB_SHA:-unknown}\`"
  echo "- Workflow run: \`${GITHUB_RUN_ID:-local}\`"
  echo "- Generated: \`$(date -u '+%Y-%m-%dT%H:%M:%SZ')\`"
} > "${output_file}"
