package dfman

import (
	"context"
	"debug/buildinfo"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
)

// Called by package hooks in the desktop user's session, never as root.
func setupPackageAgent(ctx context.Context, config, state string, out io.Writer, enable bool) error {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		return fmt.Errorf("package setup must run as the desktop user, not root")
	}
	if err := os.MkdirAll(state, 0700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(state, "package.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		return exitCode{75}
	}
	defer lock.Close()
	disabled := filepath.Join(state, "agent-disabled")
	if !enable {
		if _, err := os.Stat(disabled); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// O_EXCL also refuses dangling symlinks. Never overwrite an existing config.
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(config, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		_, err = io.WriteString(f, "notification = true\n\n[agent]\ninterval = \"10m\"\nsync_mode = \"normal\"\n\n# Add folders with dfman repo add <source> [target].\n")
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	} else if !os.IsExist(err) {
		return err
	}
	c, err := LoadConfig(config)
	if err != nil {
		_ = reportProblem(state, "config", err.Error())
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	config, err = filepath.Abs(config)
	if err != nil {
		return err
	}
	state, err = filepath.Abs(state)
	if err != nil {
		return err
	}
	if err := migratePortableBinary(exe, state); err != nil {
		return err
	}
	if err := installAgent(ctx, exe, config, state, c, out); err != nil {
		return err
	}
	if err := os.Remove(disabled); err != nil && !os.IsNotExist(err) {
		return err
	}
	return writeAtomic(filepath.Join(state, "agent-notification"), []byte(fmt.Sprint(c.Notification)), 0600)
}

func packageKind() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	b, _ := os.ReadFile(filepath.Join(filepath.Dir(exe), ".dfman-package"))
	return string(b)
}

// Avoid an older portable Go binary shadowing the system package on PATH.
// Only binaries from this Go module are moved, and a rollback copy is retained.
func migratePortableBinary(exe, state string) error {
	if runtime.GOOS == "windows" || packageKind() == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	old := filepath.Join(home, ".local", "bin", "dfman")
	resolved, err := filepath.EvalSymlinks(old)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if resolved == exe {
		return nil
	}
	info, err := buildinfo.ReadFile(resolved)
	if err != nil || info.Main.Path != "github.com/kopiro/dfman" {
		return fmt.Errorf("%s shadows the packaged executable; move it aside and sign in again", old)
	}
	backupDir := filepath.Join(state, "package-backup", time.Now().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return err
	}
	backup := filepath.Join(backupDir, "dfman")
	if err := os.Rename(old, backup); err != nil {
		return err
	}
	if err := os.Symlink(exe, old); err != nil {
		_ = os.Rename(backup, old)
		return err
	}
	return nil
}

func removePackageAgent(ctx context.Context, state string, out io.Writer) error {
	if err := writeAtomic(filepath.Join(state, "agent-disabled"), []byte("disabled"), 0600); err != nil {
		return err
	}
	if err := uninstallAgent(ctx, out); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	alias := filepath.Join(home, ".local", "bin", "dfman")
	target, err := os.Readlink(alias)
	if err == nil && target == exe {
		return os.Remove(alias)
	}
	return nil
}
