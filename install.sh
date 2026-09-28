#!/bin/sh
set -eu
# Installs the native package, including its desktop agent.
version=${DFMAN_VERSION:-latest}
case "$(uname -s)" in Darwin) platform=darwin;; Linux) platform=linux;; *) echo 'Use install.ps1 on Windows.' >&2; exit 1;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64;; x86_64) arch=amd64;; *) echo 'Unsupported architecture.' >&2; exit 1;; esac
if [ "$version" = latest ]; then
 version=$(curl -fsSL https://api.github.com/repos/kopiro/dfman/releases/latest | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
fi
case "$version" in v[0-9]* ) ;; *) echo 'Invalid release version.' >&2; exit 1;; esac
base="https://github.com/kopiro/dfman/releases/download/$version"
extension=deb
[ "$platform" != darwin ] || extension=pkg
name="dfman_${version}_${platform}_${arch}.$extension"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
curl -fsSL "$base/$name" -o "$tmp/$name"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v n="$name" '$2==n {print $1}' "$tmp/checksums.txt")
if command -v sha256sum >/dev/null 2>&1; then actual=$(sha256sum "$tmp/$name" | cut -d ' ' -f1); else actual=$(shasum -a 256 "$tmp/$name" | cut -d ' ' -f1); fi
[ -n "$expected" ] && [ "$expected" = "$actual" ] || { echo 'Checksum verification failed.' >&2; exit 1; }
if [ "$platform" = darwin ]; then
 sudo /usr/sbin/installer -pkg "$tmp/$name" -target /
else
 sudo apt-get install "$tmp/$name"
fi
printf 'Installed %s. The agent is configured automatically in your desktop session.\n' "$version"
