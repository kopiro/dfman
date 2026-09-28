#!/bin/bash
# Run only on an ephemeral CI runner: installs/removes system packages.
set -euo pipefail
version=${1:?}
arch=${2:?}
hostarch=$(uname -m)
[[ "$hostarch" != x86_64 ]] || hostarch=amd64
[[ "$hostarch" != aarch64 ]] || hostarch=arm64
[[ "$arch" == "$hostarch" ]] || { echo "Cross-built package; native installer test runs in the matching job."; exit 0; }
mkdir -p "$HOME/.config"
printf 'notification=false\n\n[agent]\ninterval="1m"\nsync_mode="reset"\n' > "$HOME/.config/dfman.conf"
cp "$HOME/.config/dfman.conf" "$HOME/.config/dfman.conf.expected"
if [[ $(uname) == Darwin ]]; then
 sudo installer -pkg "dist/dfman_${version}_darwin_${arch}.pkg" -target /
 exe=/usr/local/libexec/dfman/dfman
else
 sudo dpkg -i "dist/dfman_${version}_linux_${arch}.deb"
 exe=/usr/lib/dfman/dfman
fi
[[ $("$exe" version) == "$version" ]]
if "$exe" agent install; then echo "Removed command still works"; exit 1; fi
"$exe" agent run
cmp "$HOME/.config/dfman.conf" "$HOME/.config/dfman.conf.expected"
# Linux user managers on hosted runners may lack a graphical session; validate
# the installed login hook and leave desktop delivery to real-host validation.
if [[ $(uname) == Linux ]]; then
 test -f /etc/xdg/autostart/dfman.desktop
 sudo dpkg -r dfman
 cmp "$HOME/.config/dfman.conf" "$HOME/.config/dfman.conf.expected"
 test ! -e /usr/lib/dfman/dfman
else
 plutil -lint /Library/LaunchAgents/com.kopiro.dfman.setup.plist
fi
