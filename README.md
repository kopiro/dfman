# dfman

A native dotfiles manager for macOS, Linux, and Windows. Keep files in Git,
link them into place, and optionally synchronize them from your desktop session.

## Install

Requires Git 2.38 or newer. Download an installer from
[Releases](https://github.com/kopiro/dfman/releases) and verify its SHA-256 against
`checksums.txt`:

- **macOS:** open the `.pkg`. It installs into `/usr/local/libexec/dfman` with a
  command at `/usr/local/bin/dfman`.
- **Ubuntu:** run `sudo apt install ./dfman_<version>_linux_<arch>.deb`.
- **Windows:** run the `_setup.exe` as your normal desktop user. It installs into
  `%USERPROFILE%\.local\bin` and adds that directory to your user PATH.

Installers automatically configure the agent for the desktop user. macOS and
Ubuntu also configure new desktop sessions at login. Existing configuration is
preserved. A new installation creates an empty `~/.config/dfman.conf`; add your
folders with `dfman repo add <source> [target]`. Until then, the agent is idle.
Git authentication must already work without an interactive prompt.

macOS packages are currently **unsigned and not notarized**. Windows installers
are unsigned as well. Operating systems may display an unverified-publisher
warning. No signing keys or notarization credentials are used in CI.

The repository's `install.sh` and `install.ps1` download the native installer,
verify its checksum, and run it. Releases contain native installers only.

The Windows desktop agent needs Developer Mode for native symlinks from its
non-elevated session. Manual linking can also use symlink privilege. Links are native symlinks;
dfman never substitutes copies. Clone symlink-bearing repositories using
`git -c core.symlinks=true clone ...` with that privilege enabled.

## Configuration

Create `~/.config/dfman.conf` as TOML:

```toml
notification = true

[agent]
enabled = true
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
Intervals are whole minutes from `1m` through `24h`. Rerun the installer after
changing the interval (signing in again also applies it on macOS and Ubuntu).
Other config changes apply on the next run.

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

The native installer configures:

- macOS: `~/Library/LaunchAgents/com.kopiro.dfman.agent.plist` (Aqua session).
- Linux: `~/.config/systemd/user/dfman-agent.service` and `.timer`, attached to
  `graphical-session.target`.
- Windows: interactive Task Scheduler task `dfman-agent`, under the current user, without elevation.

Agents sync, then link only when all repositories synchronize successfully.
No-change runs are silent. Pulled/pushed data and new errors produce notifications;
identical unresolved errors are suppressed after successful delivery. Delivery
failures appear in status without changing the Git exit result. Manual sync and link
never notify. macOS requests notification permission during package setup.
Linux talks to the desktop notification service over D-Bus; Windows registers a
dfman Start Menu identity and uses native toasts. No notification utility is required.

Agent logs and status are in `~/.local/state/dfman` (or `$XDG_STATE_HOME/dfman`).
`dfman status` shows configuration, scheduler details, and run results.
Set `enabled = false` under `[agent]` to disable automatic syncing, linking,
and notifications. This takes effect on the next scheduled run; an already
running sync finishes normally. Set it back to `true` to resume. The scheduler
remains registered while disabled, so no reinstall is required.
Agents run only in signed-in desktop sessions.

To remove the entire package: use Windows Installed Apps, `sudo apt remove dfman`,
or `sudo /usr/local/libexec/dfman/uninstall` on macOS. User data is preserved.

### Updates

The agent checks for releases at most once every 24 hours when notifications are
enabled. It notifies once per newer version with installation instructions.
Update-check failures appear in status without changing the sync result.
Updates are never installed automatically.

Use `dfman self update --check` to check manually. For package installations,
download and run the newer installer. Older portable installations must also
migrate using an installer; ZIP updates are no longer published.

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
bash scripts/package-macos.sh v1.0.0 arm64
```

CI tests macOS/Linux/Windows and builds amd64/arm64 packages for each. The workflow
produces release artifacts and checksums but does not publish. Release publication
requires the three-host desktop validation gate documented in [RELEASE.md](RELEASE.md).

### Native installers and CI releases

Build the native payload first using the scripts below, then run
`scripts/package-macos.sh vX.Y.Z arm64`,
`scripts/package-linux.sh vX.Y.Z amd64`, or
`scripts/package-windows.ps1 -Version vX.Y.Z -Arch amd64`.
Windows packaging requires Inno Setup 6; Ubuntu packaging requires `dpkg-deb`.

CI builds amd64 and arm64 payloads and installers for all three platforms.
Pushing a `vX.Y.Z` tag automatically publishes all packages and checksums after
tests, builds, and installer checks pass. Manual workflow runs only build
artifacts. See [RELEASE.md](RELEASE.md) for the desktop validation gate.
