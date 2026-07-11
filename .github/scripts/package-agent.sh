#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <target> <binary> <output-dir>" >&2
  exit 2
fi

target="$1"
binary="$2"
out_dir="$3"
name="netgoat-agent-${target}"
stage="${out_dir}/.pkg-${name}/${name}"

rm -rf "${out_dir}/.pkg-${name}"
mkdir -p "${stage}"

cp "${binary}" "${stage}/"
cp README.md LICENSE config.yml "${stage}/"

if [[ -d public ]]; then
  mkdir -p "${stage}/public"
  cp -R public/. "${stage}/public/"
fi

if [[ -d ai ]]; then
  mkdir -p "${stage}/ai"
  find ai -maxdepth 1 -type f -name '*.py' -exec cp {} "${stage}/ai/" \;
fi

if [[ -f quickstart-overlay.sh ]]; then
  cp quickstart-overlay.sh "${stage}/"
fi

if [[ "${target}" == windows-* ]]; then
  archive="${out_dir}/${name}.zip"
  (cd "${out_dir}" && zip -qr "${name}.zip" "${name}")
else
  archive="${out_dir}/${name}.tar.gz"
  tar -C "${out_dir}" -czf "${archive}" "${name}"
fi

if command -v sha256sum >/dev/null 2>&1; then
  (cd "${out_dir}" && sha256sum "$(basename "${archive}")" > "$(basename "${archive}").sha256")
else
  (cd "${out_dir}" && shasum -a 256 "$(basename "${archive}")" > "$(basename "${archive}").sha256")
fi

rm -rf "${out_dir}/.pkg-${name}"
echo "Created ${archive}"
