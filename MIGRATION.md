# Bash → Go migration

1. Back up the installed `dfman` and runner/launcher files, `dfman.conf`, local
   status directory, and your existing scheduler registration. Keep repository
   recovery snapshots. Resolve unfinished Git operations first.
2. Disable the old dfman cron entry / LaunchAgent / scheduled task and wait for
   any running sync to finish. Keep unrelated scheduled jobs. Do not run both
   generations against the same repository: their lock formats differ.
3. Convert `~/.config/dfman.conf` to TOML, preserving folder order and targets.
   Follow symlinks to update the authoritative configuration. Use the example
   in this repository; there is no legacy parser. Keep authentication settings.
4. Choose `[agent] sync_mode = "normal"` for editing machines or `"reset"` for
   deliberate replicas. Reset saves local differences before replacing them.
   Set `notification = true` and your interval (default `"10m"`).
5. Extract the complete native package beside the executable. On Windows, move
   the old `dfman.cmd` Bash launcher and `dfman-auto-sync.cmd` to the backup so
   they cannot shadow `dfman.exe`. Remove obsolete runner files from PATH.
6. Run `dfman version`, `dfman config validate`, and a manual sync/link check.
   On replicas, manual sync needs `--reset`; the configuration controls agents
   only. Review `dfman status` and preserve any recovery notices.
7. Run `dfman agent install` in the signed-in desktop account. Accept macOS
   notification permission. The Windows task uses an interactive logon, replacing
   the old S4U behavior. Linux needs a running desktop notification service and
   systemd graphical session. Check `dfman agent status` after a scheduled run.
8. Run `dfman shell install zsh` if desired. Replace calls to `repo-add`,
   `repo-remove`, `repo-sync`, and `self-update` with `repo add`, `repo remove`,
   `repo sync`, and `self update`. Replace old auto-sync invocations with the
   explicit agent installation; no cron command remains.

## Roll back

Run the native `dfman agent uninstall` and wait for the current run to stop.
Restore the backed-up executable, complete old configuration and launcher/runner
files. Restore only the saved dfman scheduler entry, retaining unrelated changes
to the scheduler. Restore old status separately if needed. Keep native recovery
snapshots; restoring a tool never requires discarding repository data.
