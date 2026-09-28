package dfman

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type updateNoticeState struct {
	Checked  time.Time
	Notified string
}

// Only stable release versions are eligible; development builds never advertise
// a downgrade to the latest published release.
func releaseVersion(s string) ([3]uint64, bool) {
	var v [3]uint64
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil || p == "" {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func newerRelease(candidate, current string) bool {
	a, ok := releaseVersion(candidate)
	b, valid := releaseVersion(current)
	if !ok || !valid {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func agentUpdateNotice(ctx context.Context, current, state string, out io.Writer, notify notifier, fetch func(context.Context) (release, error), now time.Time) error {
	if _, ok := releaseVersion(current); !ok {
		return nil
	}
	path := filepath.Join(state, "update-notice.json")
	var cache updateNoticeState
	if b, err := os.ReadFile(path); err == nil {
		if err = json.Unmarshal(b, &cache); err != nil {
			return fmt.Errorf("read update notice state: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if !cache.Checked.IsZero() && now.Sub(cache.Checked) < 24*time.Hour {
		return nil
	}
	// Persist attempts as well as successes to throttle offline failures.
	cache.Checked = now
	save := func() error {
		b, err := json.Marshal(cache)
		if err != nil {
			return err
		}
		return writeAtomic(path, b, 0600)
	}
	if err := save(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r, err := fetch(ctx)
	if err != nil {
		return err
	}
	if err := reportProblem(state, "update-check", ""); err != nil {
		return err
	}
	if !newerRelease(r.Tag, current) || cache.Notified == r.Tag {
		return nil
	}
	body := r.Tag + " is available. Run dfman self update to install."
	if packageKind() != "" {
		body = r.Tag + " is available. Download the installer from github.com/kopiro/dfman/releases."
	}
	if err := notify(ctx, "dfman update available", body, hash("update")); err != nil {
		return fmt.Errorf("update notification: %w", err)
	}
	cache.Notified = r.Tag
	if err := save(); err != nil {
		return err
	}
	fmt.Fprintln(out, "Notification delivered: dfman update available", r.Tag)
	return nil
}
