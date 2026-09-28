#!/bin/bash
set -euo pipefail
version=${1:?version required}
arch=${2:?architecture required}
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Expected vX.Y.Z"; exit 1; }
[[ "$arch" == arm64 || "$arch" == amd64 ]] || exit 1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/DEBIAN" "$tmp/usr/lib/dfman" "$tmp/usr/bin" "$tmp/etc/xdg/autostart"
cp "dist/linux-$arch/dfman" "$tmp/usr/lib/dfman/dfman"
ln -s ../lib/dfman/dfman "$tmp/usr/bin/dfman"
mkdir -p "$tmp/usr/share/icons/hicolor/256x256/apps"
cp assets/dfman-256.png "$tmp/usr/share/icons/hicolor/256x256/apps/dfman.png"
cp packaging/linux/dfman.desktop "$tmp/etc/xdg/autostart/dfman.desktop"
cp packaging/linux/postinst packaging/linux/prerm "$tmp/DEBIAN/"
chmod 755 "$tmp/DEBIAN/postinst" "$tmp/DEBIAN/prerm"
cat > "$tmp/DEBIAN/control" <<CONTROL
Package: dfman
Version: ${version#v}
Section: utils
Priority: optional
Architecture: $arch
Maintainer: kopiro <kopiro@users.noreply.github.com>
Depends: git (>= 1:2.38), systemd, dbus-user-session
Homepage: https://github.com/kopiro/dfman
Description: Git dotfiles manager with desktop synchronization
 Installs a desktop-session agent. Existing user configuration is preserved.
CONTROL
dpkg-deb --root-owner-group --build "$tmp" "dist/dfman_${version}_linux_${arch}.deb"
