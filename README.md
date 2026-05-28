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
multiple repos are configured, they are linked in config order.

```bash
dfman link
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

Synchronizes configured git repos. It stages all changes, commits them as
`sync by {hostname} at YYYY-MM-DD HH:MM:SS`, pulls with rebase, then pushes. If
pull hits conflicts, `dfman` warns and leaves the repo for manual resolution.

```bash
dfman repo-sync
```

Sync only one configured source:

```bash
dfman repo-sync --repo '~/.dotfiles'
```

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

Updates the installed `dfman` executable from GitHub.

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
