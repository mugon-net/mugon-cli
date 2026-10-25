#!/bin/sh
# Installs the mugon CLI. Served at https://tools.mugon.net/cli/install.sh.
#   MUGON_INSTALL    install folder (default ~/.mugon/bin)
#   MUGON_TOOLS_URL  tools host (default https://tools.mugon.net)
# The index (/cli/latest.json) has one line per platform, e.g.
#   "linux-amd64": {"url": "https://...", "sha256": "..."},
set -e

bold=$(printf '\033[1m')
green=$(printf '\033[32m')
red=$(printf '\033[31m')
reset=$(printf '\033[0m')

info()  { printf "%s\n" "${bold}$*${reset}"; }
ok()    { printf "%s\n" "${green}$*${reset}"; }
error() { printf "%s\n" "${red}error: $*${reset}" >&2; exit 1; }

case "$(uname -s)" in
  Linux*)  os=linux ;;
  Darwin*) os=darwin ;;
  *)       error "unsupported OS: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64)  arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)               error "unsupported architecture: $(uname -m)" ;;
esac

tools_url="${MUGON_TOOLS_URL:-https://tools.mugon.net}"
platform="${os}-${arch}"

index=$(curl -fsSL "${tools_url}/cli/latest.json") || error "failed to download ${tools_url}/cli/latest.json"
version=$(printf '%s\n' "$index" | sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' | head -n 1)
entry=$(printf '%s\n' "$index" | grep "\"${platform}\"" | head -n 1)
url=$(printf '%s\n' "$entry" | sed -n 's/.*"url": *"\([^"]*\)".*/\1/p')
sha256=$(printf '%s\n' "$entry" | sed -n 's/.*"sha256": *"\([^"]*\)".*/\1/p')

[ -n "$version" ] && [ -n "$url" ] && [ -n "$sha256" ] || error "no mugon release for ${platform}"

if command -v sha256sum >/dev/null 2>&1; then
  hash_file() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
  hash_file() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
  error "sha256sum or shasum is required"
fi

install_dir="${MUGON_INSTALL:-$HOME/.mugon/bin}"
mkdir -p "$install_dir"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "Downloading mugon ${version}..."
curl -fsSL "$url" -o "$tmp/mugon.tar.gz" || error "failed to download $url"
[ "$(hash_file "$tmp/mugon.tar.gz")" = "$sha256" ] || error "checksum mismatch for $url"

tar -xzf "$tmp/mugon.tar.gz" -C "$tmp"
cp "$tmp/mugon" "$install_dir/mugon"
chmod +x "$install_dir/mugon"

ok "Installed mugon ${version} to ${install_dir}/mugon"

case "$SHELL" in
  */zsh)  profile="$HOME/.zshrc" ;;
  */fish) profile="$HOME/.config/fish/conf.d/mugon.fish" ;;
  *)      profile="$HOME/.bashrc" ;;
esac

if ! grep -qF "$install_dir" "$profile" 2>/dev/null; then
  printf '\n# mugon\nexport PATH="$PATH:%s"\n' "$install_dir" >> "$profile"
  echo ""
  info "Added ${install_dir} to PATH in ${profile}"
fi

# Only reaches the current shell when the script is sourced, not when piped through sh.
export PATH="$PATH:$install_dir"

echo ""
ok "mugon ${version} installed"
info "Run 'source ${profile}' or open a new shell to start using mugon"
