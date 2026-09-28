package dfman

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gofrs/flock"
)

// Called by package hooks in the desktop user's session, never as root.
func setupPackageAgent(ctx context.Context, config, state string, out io.Writer) error {
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
	// O_EXCL also refuses dangling symlinks. Never overwrite an existing config.
	if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(config, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		_, err = io.WriteString(f, "notification = true\n\n[agent]\nenabled = true\ninterval = \"10m\"\nsync_mode = \"normal\"\n\n# Add folders with dfman repo add <source> [target].\n")
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
	if err := installAgent(ctx, exe, config, state, c, out); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(state, "agent-notification"), []byte(fmt.Sprint(c.Notification)), 0600)
}

func removePackageAgent(ctx context.Context, state string, out io.Writer) error {
	return uninstallAgent(ctx, out)
}
