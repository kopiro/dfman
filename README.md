# dfman

Small dotfiles manager for copying files into a dotfiles directory and linking
them back into a target directory.

## Installation

### macOS and Linux

Install `dfman` with `wget`:

```bash
mkdir -p "$HOME/.local/bin"
wget -O "$HOME/.local/bin/dfman" https://raw.githubusercontent.com/kopiro/dfman/main/dfman
chmod 0755 "$HOME/.local/bin/dfman"
```

Make sure `$HOME/.local/bin` is in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Windows

Install Git for Windows, then run in PowerShell:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/kopiro/dfman/main/install.ps1 -OutFile "$env:TEMP\dfman-install.ps1"
& "$env:TEMP\dfman-install.ps1"
```

The installer adds `%USERPROFILE%\.local\bin` to your user PATH. Open a new
terminal and use `dfman` from PowerShell, cmd, or Git Bash. No WSL is needed.
Enable Windows Developer Mode or use an elevated terminal to create symlinks.
dfman uses native Windows symlinks and fails if it cannot create them; it never
silently substitutes copies. This follows the
[MSYS2 native symlink behavior](https://www.msys2.org/docs/symlinks/).

Configuration remains `~/.config/dfman.conf`, under your Windows user profile.
Use `~/.dotfiles`, `C:/Users/name/dotfiles`, or Git Bash `/c/Users/name/dotfiles`
paths. Configuration entries cannot contain whitespace or `#`; `~` works even
when the user profile path contains spaces. Repositories must contain
Windows-compatible filenames. When cloning repositories containing symlinks,
use `git -c core.symlinks=true clone ...` with symlink permission enabled.

For unattended runs, configure Task Scheduler to run as the repository owner
every ten minutes, whether logged on or not, and avoid overlapping instances.
Use `cmd.exe /d /c` with this command (substitute your profile path):

```bat
call C:\Users\name\.local\bin\dfman.cmd sync && call C:\Users\name\.local\bin\dfman.cmd link
```

The task needs non-interactive Git authentication and permission to create
symlinks. `sync` commits and pushes local changes as well as pulling updates.

## Configuration

`dfman` reads sources from `$HOME/.config/dfman.conf`.

Create an initial config with `repo-add`:

```bash
mkdir -p "$HOME/.dotfiles"
dfman repo-add '~/.dotfiles'
```

Use the path where your dotfiles already live instead of `~/.dotfiles` if
needed.

Each non-comment line contains a source directory and an optional target
directory:

```text
# source [target]
~/.work-dotfiles
~/.dotfiles
```

Repo order matters: sources earlier in the file have higher priority because
they are linked first. On a work computer, you might configure
`~/.work-dotfiles` before `~/.dotfiles`; on a personal computer, you might only
configure `~/.dotfiles`.

If target is not provided, `dfman` uses `$HOME`.
Paths must be absolute or start with `~`.

## Commands

### `dfman link`

Links files from configured source repos into their target directories. When
multiple repos are configured, they are linked in config order. Existing valid
links are only printed when verbose output is enabled.

```bash
dfman link
```

Show existing valid links:

```bash
dfman link -v
```

Link only one configured source:

```bash
dfman link --repo '~/.work-dotfiles'
```

Preview changes without writing files:

```bash
dfman link --dry-run
```

Replace conflicting files or symlinks interactively:

```bash
dfman link -f
```

### `dfman repo-add`

Adds a source repo to `$HOME/.config/dfman.conf`. The target is optional and
defaults to `$HOME`.

```bash
dfman repo-add '~/.work-dotfiles' '~'
```

### `dfman repo-rm`

Removes a configured source from `$HOME/.config/dfman.conf`.

```bash
dfman repo-rm '~/.work-dotfiles'
```

### `dfman repo-sync`

Synchronizes configured Git repos. It fetches the matching branch from `origin`,
stages local changes, commits them as `sync by {hostname} at YYYY-MM-DD HH:MM:SS`,
merges compatible remote changes, then pushes. Git 2.38 or newer is required for
merge preparation. `dfman sync` is an alias of `dfman repo-sync`.

Conflicts are detected before changing the active checkout. Local work remains
committed, the working files remain usable, and the command exits with status 2.
Resolve the differences manually, then run sync again. Existing unfinished Git
operations are never staged or committed automatically. A per-repository lock
prevents overlapping dfman syncs; stale locks require manual inspection before
removal. Repositories with submodules are not synchronized automatically.

Exit codes: 0 success, 1 operational failure, 2 conflict/manual Git resolution,
75 another sync holds the lock. A failed push preserves local commits for retry.
For multi-repository runs, conflicts take precedence over other failures.

```bash
dfman repo-sync
```

Sync only one configured source:

```bash
dfman repo-sync --repo '~/.dotfiles'
```

Use a specific SSH key for pull and push:

```bash
dfman sync --repo '~/.dotfiles' --ssh-key '~/.ssh/id_ed25519_work'
```

For an explicitly designated replica, replace local state with the default
branch on `origin`. Before any change, dfman saves the working files (including
untracked and ignored files), the Git index, and references protecting the old
HEAD and the branch being replaced. If backup creation fails, reset stops.
An already clean, aligned replica needs no new snapshot.

Snapshots are stored in the repository's Git directory under
`dfman-recovery/<timestamp>-<pid>/`. They contain private data and have restricted
permissions. They are not uploaded, automatically expired, or deleted by reset.
Incoming remote updates also get a snapshot; only discarded local differences
produce a reset notification. Ignored files are not cleaned; a checkout that
would overwrite one stops instead. Avoid concurrent non-dfman writers during
sync/reset: repository locks only serialize dfman processes.

Reset refuses unfinished merges/rebases, just like ordinary sync. It remains a
replacement operation, not a merge:

```bash
dfman sync --reset
```

Reset only one configured source:

```bash
dfman sync --reset --repo '~/.dotfiles'
```

### Recovering a reset

The command prints the snapshot directory and recovery reference. To inspect
saved commits, use `git log refs/dfman/recovery/<id>/head`. Create a new branch
from that reference when ready to recover committed work. A `/branch` reference
also preserves the former tip of the branch replaced by reset, if it existed.

Extract `<snapshot>/files.tar` into a separate empty directory to inspect
uncommitted and untracked files, then copy only the files you want back. Do not
extract it blindly over active dotfiles. `index` preserves the previous staging
state; `README` records repository and commit identities. After recovery, remove
unneeded snapshots and their `refs/dfman/recovery/<id>/*` refs explicitly.

### Scheduled synchronization and terminal notices

Install the optional `dfman-auto-sync` Bash runner beside dfman at
`~/.local/bin/dfman-auto-sync` with executable permissions. It runs sync and
links only after success, and saves local status. It sends no external
notifications and needs no notification service or credentials.

For an editing machine:

```cron
*/10 * * * * ~/.local/bin/dfman-auto-sync
```

For an explicitly designated replica, append `--reset`. Existing working
intervals may be kept. `--repo` and `--ssh-key` are forwarded to sync; linking
respects `--repo`. Windows Task Scheduler can invoke the runner with Git Bash.

The runner writes its latest log to `~/.local/state/dfman/sync.log`. Run
`dfman status` to see the last scheduled result, unresolved problems, and
recovery paths. `dfman status --ack` dismisses informational reset backup
notices; it never clears unresolved failures. Successful syncs clear failures
for the repositories involved. State and backups stay on the local machine.

Install the interactive terminal hook with:

```sh
dfman shell install
```

Automatic integration currently supports **zsh**. Pass `zsh` explicitly if your
`SHELL` environment identifies another shell. The installer honors `ZDOTDIR`,
preserves `.zshrc` symlinks by editing their resolved source, and replaces only
its marked block. Running it repeatedly adds no duplicates. It also migrates
the previous manual `dfman.zsh` source line. Open a new terminal afterward, or
run `eval "$(dfman shell init zsh)"` in the current one.

`dfman shell status` prints actual unresolved errors and backup notices, with
nothing printed when healthy. The installed hook calls it before each prompt
and suppresses unchanged messages within that shell session. New sessions show
unresolved problems again. `dfman status` remains the complete report.

`dfman shell uninstall` removes only the managed startup integration. Already
open terminals can remove the hook with
`add-zsh-hook -d precmd _dfman_prompt_notice`, or simply reopen. Status files,
backups, and scheduled synchronization are retained.

Shell status also schedules a **background update check at most every six
hours**. It reads cached results immediately and never waits for the network.
The scheduled runner can refresh the same cache without an open terminal.
Checks have bounded network timeouts; offline failures are quiet and do not
change the installed executable. The latest commit is resolved through GitHub's
public API, then downloaded by its immutable SHA to avoid stale branch caches.
API rate limits leave the previous result untouched until the next check. Run `dfman self-update --check` to request a fresh check.
When the published script differs, the prompt proposes `dfman self-update`;
it never applies the update automatically. Comparison uses script contents,
not version numbers: an unpublished local edit can also differ from the
published copy. A result for a different installed file is ignored.

Overrides: `DFMAN_BIN`, `XDG_STATE_HOME`. `DFMAN_REPORT_DIR` is the internal
sync report destination used by the runner. Interrupted runner locks appear
in status and require inspection/removal rather than automatic lock stealing.

### `dfman create`

Imports an existing file into a specific configured source. Nested paths are
stored using `#` separators. After importing and optionally removing the
original file, run `dfman link` when you are ready to create the symlink.

```bash
dfman create --repo '~/.dotfiles' "$HOME/.zshrc"
```

### `dfman doctor`

Finds broken symlinks inside configured source repos.

```bash
dfman doctor
```

Check only one configured source:

```bash
dfman doctor --repo '~/.work-dotfiles'
```

### `dfman self-update`

Updates the installed `dfman` executable from GitHub. Use `--check` to compare
with the published copy without installing it.

```bash
dfman self-update
```

## Nested Paths

`dfman` stores nested target paths as flat filenames by replacing `/` with `#`.
For example, this source file:

```text
$HOME/.dotfiles/.config#git#config
```

links to:

```text
$HOME/.config/git/config
```
