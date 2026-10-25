#!/usr/bin/env bash
# Publishes the CLI install scripts and /cli/latest.json to the tools host (a Bunny storage zone).
# Run after goreleaser has released <version> (reads dist/checksums.txt).
#
# Usage: scripts/publish-tools.sh <version>            e.g. scripts/publish-tools.sh v0.5.0
# Environment:
#   TOOLS_STORAGE_ZONE  storage zone name
#   TOOLS_STORAGE_KEY   zone password (write access)
#   TOOLS_STORAGE_HOST  default storage.bunnycdn.com (use the region host for non-DE zones)
#   GITHUB_REPOSITORY   default mugon-net/mugon-cli
#   DRY_RUN=1           write dist/tools/ only, upload nothing
set -euo pipefail

tag="${1:?version}"
version="${tag#v}"
repo="${GITHUB_REPOSITORY:-mugon-net/mugon-cli}"
checksums="dist/checksums.txt"
out="dist/tools/cli"
mkdir -p "$out"

platform_key() {
  local os arch
  case "$1" in
    *_linux_*) os=linux ;; *_darwin_*) os=darwin ;; *_windows_*) os=windows ;; *) return 1 ;;
  esac
  case "$1" in
    *_amd64.*) arch=amd64 ;; *_arm64.*) arch=arm64 ;; *) return 1 ;;
  esac
  echo "${os}-${arch}"
}

lines=()
while read -r sha file; do
  case "$file" in *.tar.gz | *.zip) ;; *) continue ;; esac
  key="$(platform_key "$file")" || continue
  lines+=("    \"${key}\": {\"url\": \"https://github.com/${repo}/releases/download/${tag}/${file}\", \"sha256\": \"${sha}\"}")
done < "$checksums"

[ "${#lines[@]}" -gt 0 ] || { echo "no archives found in ${checksums}" >&2; exit 1; }

{
  printf '{\n  "version": "%s",\n  "assets": {\n' "$version"
  for position in "${!lines[@]}"; do
    if [ "$position" -lt $((${#lines[@]} - 1)) ]; then printf '%s,\n' "${lines[$position]}"; else printf '%s\n' "${lines[$position]}"; fi
  done
  printf '  }\n}\n'
} > "${out}/latest.json"
cp scripts/install.sh scripts/install.ps1 "$out/"

if [ "${DRY_RUN:-}" = "1" ]; then
  echo "dry run: wrote ${out}/ (nothing uploaded)"
  exit 0
fi

: "${TOOLS_STORAGE_ZONE:?}" "${TOOLS_STORAGE_KEY:?}"
host="${TOOLS_STORAGE_HOST:-storage.bunnycdn.com}"
for file in install.sh install.ps1 latest.json; do
  curl --fail --silent --show-error --request PUT \
    --header "AccessKey: ${TOOLS_STORAGE_KEY}" --header "Content-Type: application/octet-stream" \
    --data-binary "@${out}/${file}" "https://${host}/${TOOLS_STORAGE_ZONE}/cli/${file}"
  echo "uploaded cli/${file}"
done
