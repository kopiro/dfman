# Release gate

Publish v1.0.0 only when all checks below pass. Building artifacts alone does not
satisfy the desktop validation gate.

- Automated tests and native package builds for macOS, Linux, Windows.
- amd64 and arm64 ZIP packages containing the executable and required helper.
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

Use the workflow_dispatch version input to build release artifacts. The workflow
never publishes automatically. Keep dated execution evidence in the private
HomeLab task record rather than this public repository.
