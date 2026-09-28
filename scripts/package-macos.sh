#!/bin/bash
set -euo pipefail
version=${1:?version required}
arch=${2:?architecture required}
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Expected vX.Y.Z"; exit 1; }
[[ "$arch" == arm64 || "$arch" == amd64 ]] || exit 1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
root="$tmp/root"
mkdir -p "$root/usr/local/libexec/dfman" "$root/usr/local/bin" "$root/Library/LaunchAgents"
cp -R "dist/darwin-$arch/." "$root/usr/local/libexec/dfman/"
cp packaging/macos/uninstall.sh "$root/usr/local/libexec/dfman/uninstall"
chmod 755 "$root/usr/local/libexec/dfman/uninstall"
ln -s ../libexec/dfman/dfman "$root/usr/local/bin/dfman"
cp packaging/macos/com.kopiro.dfman.setup.plist "$root/Library/LaunchAgents/"
pkgbuild --analyze --root "$root" "$tmp/components.plist"
python3 - "$tmp/components.plist" <<'PYTHON'
import plistlib, sys
path = sys.argv[1]
with open(path, "rb") as stream:
    components = plistlib.load(stream)
for component in components:
    component["BundleIsRelocatable"] = False
with open(path, "wb") as stream:
    plistlib.dump(components, stream)
PYTHON
pkgbuild --root "$root" --component-plist "$tmp/components.plist" --identifier com.kopiro.dfman --version "${version#v}" --install-location / --scripts packaging/macos/scripts "$tmp/component.pkg"
hostarch=$arch
[[ "$arch" != amd64 ]] || hostarch=x86_64
cat > "$tmp/distribution.xml" <<XML
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="2">
<title>dfman</title>
<options customize="never" hostArchitectures="$hostarch"/>
<domains enable_localSystem="true"/>
<installation-check script="checkOS()"/>
<script><![CDATA[
function checkOS() {
 if (system.compareVersions(system.version.ProductVersion, '11.0') < 0) {
  my.result.message = 'dfman requires macOS 11 or later.';
  my.result.type = 'Fatal'; return false;
 }
 return true;
}
]]></script>
<choices-outline><line choice="default"/></choices-outline>
<choice id="default" visible="false"><pkg-ref id="com.kopiro.dfman"/></choice>
<pkg-ref id="com.kopiro.dfman" version="${version#v}">component.pkg</pkg-ref>
</installer-gui-script>
XML
productbuild --distribution "$tmp/distribution.xml" --package-path "$tmp" "dist/dfman_${version}_darwin_${arch}.pkg"
