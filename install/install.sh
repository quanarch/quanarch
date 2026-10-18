#!/bin/sh
# Installs qu and quanarch on Linux and macOS.
#
#   curl -fsSL https://quanarch.org/install.sh | sh
#
# QU_VERSION      a release such as 0.2.0 (default: the latest)
# QU_INSTALL_DIR  where the binaries go (default: ~/.local/bin)
set -eu

repo=quanarch/quanarch
dir=${QU_INSTALL_DIR:-$HOME/.local/bin}
version=${QU_VERSION:-}

fail() {
  echo "error: $*" >&2
  exit 1
}

for tool in curl tar; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux | darwin) ;;
  *) fail "unsupported system: $os (on Windows, use install.ps1)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported processor: $(uname -m)" ;;
esac

if [ -z "$version" ]; then
  latest=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  version=${latest##*/}
fi
version=${version#v}

archive="quanarch_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/v$version"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading qu $version for $os/$arch…"
curl -fsSL "$base/$archive" -o "$tmp/$archive" || fail "could not download $base/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "could not download the checksums"

expected=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1)
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
  actual=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ -n "$expected" ] && [ "$expected" = "$actual" ] || fail "checksum mismatch for $archive"

tar -xzf "$tmp/$archive" -C "$tmp" quanarch qu
mkdir -p "$dir"
for bin in quanarch qu; do
  cp "$tmp/$bin" "$dir/$bin"
  chmod 755 "$dir/$bin"
done

echo "Installed qu and quanarch $version to $dir"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "Add it to your PATH:  export PATH=\"$dir:\$PATH\"" ;;
esac
