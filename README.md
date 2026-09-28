# dfman

A native dotfiles manager for macOS, Linux, and Windows. Keep files in Git,
link them into place, and optionally synchronize them from your desktop session.

## Install

Requires Git 2.38 or newer. Download the package for your OS and architecture
from [Releases](https://github.com/kopiro/dfman/releases), verify its SHA-256
against `checksums.txt`, and extract **all** components into `~/.local/bin`
(or another directory on PATH). Keep the notification helper beside `dfman`.

Alternatively, download and inspect the repository's `install.sh` (macOS/Linux)
or `install.ps1` (Windows), then run it. These installers fetch versioned release
packages and verify checksums. `DFMAN_VERSION=v1.0.0` / `-Version v1.0.0` selects
a particular release. Windows installation adds the directory to your user PATH.

Installing the executable never installs or enables an agent. Git authentication
must already work without an interactive prompt. Existing SSH configuration,
credential helpers, and repository-local Git settings are respected.

The Windows desktop agent needs Developer Mode for native symlinks from its
non-elevated session. Manual linking can also use symlink privilege. Links are native symlinks;
dfman never substitutes copies. Clone symlink-bearing repositories using
`git -c core.symlinks=true clone ...` with that privilege enabled.

## Configuration

Create `~/.config/dfman.conf` as TOML:

```toml
notification = true

[agent]
interval = "10m"
sync_mode = "normal"

[[folders]]
source = "~/.ko-prefs/dotfiles"
target = "~"

[[folders]]
source = "~/.prefs/dotfiles"
```

The first folder wins when link destinations overlap. `target` defaults to `~`.
Paths must be absolute or start with `~`; spaces are supported. On Windows,
use forward slashes in double-quoted strings or literal TOML strings such as
`source = 'C:\Users\name\dotfiles'`. Git Bash `/c/...` paths are not native paths.
Unknown options, invalid intervals, and legacy line-based configurations are
rejected before synchronization. Run `dfman config validate` after editing.

`notification` affects agents only. `sync_mode` is `normal` or `reset`.
Intervals are whole minutes from `1m` through `24h`. Run `dfman agent install`
again after changing the interval. Other config changes apply on the next run.

## Commands

```text
dfman repo add <source> [target]
dfman repo remove <source>
dfman repo sync [--repo <source>] [--reset] [--ssh-key <path>]
dfman sync [--repo <source>] [--reset] [--ssh-key <path>]
dfman link [--repo <source>] [--dry-run] [--force] [--verbose]
dfman list [--repo <source>]
dfman create <absolute-path> --repo <source>
dfman doctor [--repo <source>]
dfman status [--ack]
dfman config validate
dfman version
dfman self update [--check]
dfman agent install|uninstall|run|status
dfman shell install|uninstall|init|status [zsh]
```

Global `--config <file>` and `--state-dir <directory>` support isolated setups.
Hyphenated command names and the old auto-sync runner have been removed.

### Linking

Top-level source entries are linked into the target. `#` in a filename represents
a directory boundary: `.config#example` links to `~/.config/example`. `.git` is
excluded. Existing correct links remain untouched. Conflicting files, directories,
or links are skipped; `--force` asks before replacing each one. `create` copies a
file or directory into the selected source, then asks whether to remove the original.
`doctor` reports broken source symlinks.

### Synchronization and recovery

Normal sync fetches the current branch, commits local changes, checks for conflicts,
merges remote changes, then pushes. Conflicts leave local work committed and do not
leave a new unresolved merge. Failed pushes retain local commits for the next run.
Multiple folders within one Git repository are synchronized once. Existing merges,
rebases, or conflicts must be resolved manually; repositories with submodules are
refused. Git commands have a two-minute timeout and non-interactive prompts.

`sync --reset` explicitly makes a replica match origin's advertised default branch.
Before changing local files or branch tips, it archives the working tree (including
ignored and untracked files), saves the index, and creates recovery refs. Snapshots
live under the repository's Git directory in `dfman-recovery/`. Each contains a
README with the old HEAD and recovery reference. Inspect the tar archive in a
separate directory before restoring selected files; recover committed history with
`git branch recovered <recovery-ref>/head`. Snapshots are never automatically deleted.
Ignored files remain in place; ignored paths that block checkout cause a safe failure.

`dfman status` reports results, notification failures, and saved local differences.
`status --ack` dismisses notices, preserving recovery data. OS locks release when a
process exits. Exit codes: `0` success, `1` error, `2` Git conflict, `75` already running.

### Desktop agent

Run `dfman agent install` explicitly from the signed-in desktop account. It installs:

- macOS: `~/Library/LaunchAgents/com.kopiro.dfman.agent.plist` (Aqua session).
- Linux: `~/.config/systemd/user/dfman-agent.service` and `.timer`, attached to
  `graphical-session.target`.
- Windows: interactive Task Scheduler task `dfman-agent`, under the current user, without elevation.

Agents sync, then link only when all repositories synchronize successfully.
No-change runs are silent. Pulled/pushed data and new errors produce notifications;
identical unresolved errors are suppressed after successful delivery. Delivery
failures appear in status without changing the Git exit result. Manual sync and link
never notify. macOS requests notification permission during explicit installation.
Linux talks to the desktop notification service over D-Bus; Windows registers a
dfman Start Menu identity and uses native toasts. No notification utility is required.

Agent logs and status are in `~/.local/state/dfman` (or `$XDG_STATE_HOME/dfman`).
`agent status` shows both scheduler and run status. `agent uninstall` removes only
its registration and Windows notification shortcut; it leaves configuration, links,
repositories, and recovery data intact. Agents run only in signed-in desktop sessions.

### Shell integration and updates

`dfman shell install zsh` adds a managed startup block, preserving a symlinked
`.zshrc`. Prompt notices show unresolved problems once per changed message.
Healthy prompts are silent. Use `dfman self update --check` to check releases;
`dfman self update` downloads and verifies the full platform package. On Windows,
a separate process replaces the executable after it exits; `dfman-update.log`
beside the executable records the result. Reinstall the agent after updating.

## Migrating from Bash

This is a breaking release. See [MIGRATION.md](MIGRATION.md) for configuration,
command, scheduler, and rollback steps. Do not point a raw-script updater at the
native executable.

## Build and test

Go (version in `go.mod`) and Git are needed for the shared core. Native macOS
packages need Xcode command-line tools; Windows packages need the Visual Studio
C++ tools and Windows SDK. Linux notification support is pure Go.

```sh
go test -race ./...
go vet ./...
bash scripts/build-macos.sh dist/darwin-arm64 arm64
# Linux: GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/linux-amd64/dfman ./cmd/dfman
# Windows developer shell: ./scripts/build-windows.ps1 -Arch amd64
python3 scripts/package.py v1.0.0 darwin-arm64
```

CI tests macOS/Linux/Windows and builds amd64/arm64 packages for each. The workflow
produces release artifacts and checksums but does not publish. Release publication
requires the three-host desktop validation gate documented in [RELEASE.md](RELEASE.md).
