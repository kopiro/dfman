#!/bin/bash
set -euo pipefail
[[ $(id -u) == 0 ]] || { echo "Run with sudo."; exit 1; }
# Remove the login hook first so it cannot recreate the agent.
rm -f /Library/LaunchAgents/com.kopiro.dfman.setup.plist
while read -r user uid; do
 [[ "$uid" -ge 500 && "$user" != _* ]] || continue
 home=$(dscl . -read "/Users/$user" NFSHomeDirectory | cut -d ' ' -f2-)
 launchctl bootout "gui/$uid/com.kopiro.dfman.setup" 2>/dev/null || true
 launchctl asuser "$uid" sudo -H -u "$user" env HOME="$home" /usr/local/libexec/dfman/dfman _package remove || {
  echo "Could not remove agent for $user; uninstall stopped." >&2
  exit 1
 }
done < <(dscl . -list /Users UniqueID)
if [[ $(readlink /usr/local/bin/dfman) == ../libexec/dfman/dfman ]]; then rm /usr/local/bin/dfman; fi
rm -rf /usr/local/libexec/dfman
pkgutil --forget com.kopiro.dfman >/dev/null
echo "dfman removed. User configuration and repositories were preserved."
