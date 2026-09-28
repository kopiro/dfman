# Release gate

Push a release tag only when all checks below pass. Building artifacts alone does not
satisfy the desktop validation gate.

- Automated tests and native package builds for macOS, Linux, Windows.
- amd64 and arm64 ZIPs plus macOS PKG, Ubuntu DEB, and Windows EXE installers.
- Fresh install, reinstall, and uninstall preserve user configuration and data.
- SHA-256 manifest covering every package.
- OttoMini: isolated Git/link tests, LaunchAgent install/reinstall/uninstall,
  configured interval, native UserNotifications delivery, error reporting,
  real configuration migration and scheduled sync in normal mode.
- KoBuntu: equivalent tests using the systemd graphical session and D-Bus,
  preserving reset mode.
- KoWin: equivalent tests using an interactive Task Scheduler session and
  native toast history, preserving reset mode and repository-local SSH settings.
- Old scheduler removed without duplicate work; backups retained on all hosts.
- HomeLab records updated after successful migration.

Use the workflow_dispatch version input to build candidate artifacts without
publishing. Pushing a stable `vX.Y.Z` tag publishes automatically after CI succeeds.
The release starts as a draft and is published only after every asset is uploaded.
Published releases are immutable on workflow reruns. macOS packages are unsigned
and not notarized; Windows installers are unsigned. CI uses its scoped GitHub
token for publication and requires no signing keys. Keep dated execution evidence in the private
HomeLab task record rather than this public repository.
