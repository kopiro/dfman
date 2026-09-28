package dfman

import (
	"context"
	"fmt"
	"github.com/gofrs/flock"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type notifier func(context.Context, string, string, string) error

func runAgent(ctx context.Context, c Config, state string, out io.Writer, notify notifier, version string) error {
	if !c.Agent.Enabled {
		fmt.Fprintln(out, "Automatic sync is disabled.")
		return nil
	}
	if e := os.MkdirAll(state, 0700); e != nil {
		return e
	}
	lock := flock.New(filepath.Join(state, "agent.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return exitCode{75}
	}
	defer lock.Close()
	if e = writeAtomic(filepath.Join(state, "agent-notification"), []byte(fmt.Sprint(c.Notification)), 0600); e != nil {
		return e
	}
	if e = reportProblem(state, "config", ""); e != nil {
		return e
	}
	log, e := os.OpenFile(filepath.Join(state, "agent.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer log.Close()
	w := io.MultiWriter(log, out)
	fmt.Fprintln(w, "Started:", time.Now().Format(time.RFC3339))
	var folders []Folder
	if len(c.Folders) > 0 {
		folders, e = c.Selected("")
		if e != nil {
			return e
		}
	} else {
		fmt.Fprintln(w, "No folders configured; agent is idle.")
	}
	results := Sync(ctx, folders, c.Agent.SyncMode == "reset", "")
	code := resultCode(results)
	if e = recordResults(state, results); e != nil {
		return e
	}
	if code == 0 && len(folders) > 0 {
		e = Link(folders, LinkOptions{Input: strings.NewReader(""), Output: w})
		detail := ""
		if e != nil {
			detail = e.Error()
			code = 1
			results = append(results, Result{Repo: "link", Kind: "error", Detail: detail})
		}
		if err := reportProblem(state, "link", detail); err != nil {
			return err
		}
	}
	var notifyErrors []string
	delivered := false
	for _, r := range results {
		fmt.Fprintf(w, "%s: %s — %s\n", r.Repo, r.Kind, r.Detail)
		key := hash(r.Repo)
		seenPath := filepath.Join(state, "notifications", key)
		if r.Kind == "busy" {
			continue
		}
		title, body := "", ""
		if r.Kind == "ok" {
			os.Remove(seenPath)
			title, body = transferNotice(r)
		} else {
			title = "Sync needs attention"
			body = "Run dfman status for details."
		}
		if !c.Notification || title == "" {
			continue
		}
		body = filepath.Base(r.Repo) + ": " + body
		fingerprint := hash(r.Kind + "\n" + r.Detail)
		old, _ := os.ReadFile(seenPath)
		if r.Kind != "ok" && string(old) == fingerprint {
			continue
		}
		if err := notify(ctx, title, body, key); err != nil {
			notifyErrors = append(notifyErrors, err.Error())
		} else {
			delivered = true
			fmt.Fprintf(w, "Notification delivered: %s\n", title)
			if r.Kind != "ok" {
				if err := writeAtomic(seenPath, []byte(fingerprint), 0600); err != nil {
					notifyErrors = append(notifyErrors, err.Error())
				}
			}
		}
	}
	if delivered || len(notifyErrors) > 0 {
		if e = reportProblem(state, "notification", strings.Join(notifyErrors, "\n")); e != nil {
			return e
		}
	}
	if c.Notification {
		if err := agentUpdateNotice(ctx, version, state, w, notify, latestRelease, time.Now()); err != nil {
			fmt.Fprintln(w, "Update check:", err)
			_ = reportProblem(state, "update-check", err.Error())
		}
	}
	fmt.Fprintf(w, "Finished: exit=%d\n", code)
	if code != 0 {
		return exitCode{code}
	}
	return nil
}
func helperPath(name string) (string, error) {
	exe, e := os.Executable()
	if e != nil {
		return "", e
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return "", e
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if _, e = os.Stat(p); e != nil {
		return "", fmt.Errorf("native notification component missing: %s; reinstall the complete release package", p)
	}
	return p, nil
}

// Keep the last valid notification preference so broken TOML can still be reported.
func agentConfigError(ctx context.Context, state string, cause error, out io.Writer) error {
	if e := reportProblem(state, "config", cause.Error()); e != nil {
		return e
	}
	b, _ := os.ReadFile(filepath.Join(state, "agent-notification"))
	if string(b) != "true" {
		return cause
	}
	fingerprint := hash(cause.Error())
	seen := filepath.Join(state, "notifications", hash("config"))
	old, _ := os.ReadFile(seen)
	if string(old) != fingerprint {
		if e := desktopNotify(ctx, "Configuration needs attention", "Run dfman config validate for details.", hash("config")); e != nil {
			reportProblem(state, "notification", e.Error())
		} else {
			writeAtomic(seen, []byte(fingerprint), 0600)
			reportProblem(state, "notification", "")
			fmt.Fprintln(out, "Notification delivered: Configuration needs attention")
		}
	}
	return cause
}
