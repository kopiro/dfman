# dfman

Small dotfiles manager for copying files into a dotfiles directory and linking
them back into a target directory.

## Installation

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
~/.dotfiles
```

If target is not provided, `dfman` uses `$HOME`.
Paths must be absolute or start with `~`.

## Usage

Link all configured dotfiles in config order:

```bash
dfman link
```

Link a single configured source:

```bash
dfman link --repo '~/.work-dotfiles'
```

Preview link changes without writing files:

```bash
dfman link --dry-run
```

Replace conflicting files or symlinks interactively:

```bash
dfman link -f
```

Add a specific source with an optional target:

```bash
dfman repo-add '~/.work-dotfiles' '~'
```

Remove a configured source:

```bash
dfman repo-rm '~/.work-dotfiles'
```

Commit, pull, and push all configured repos:

```bash
dfman repo-sync
```

Sync a single configured repo:

```bash
dfman repo-sync --repo '~/.dotfiles'
```

`repo-sync` stages all changes, commits them as
`Auto-Sync (YYYY-MM-DD HH:MM:SS)`, pulls with rebase, then pushes. If pull hits
conflicts, `dfman` warns and leaves the repo for manual resolution.

Import an existing file into a specific configured source:

```bash
dfman create --repo '~/.dotfiles' "$HOME/.zshrc"
```

Find broken symlinks in all configured source directories:

```bash
dfman doctor
```

Find broken symlinks in a single configured source:

```bash
dfman doctor --repo '~/.work-dotfiles'
```

Update `dfman` from GitHub:

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
