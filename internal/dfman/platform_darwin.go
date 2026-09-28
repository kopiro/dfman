package dfman

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const agentLabel = "com.kopiro.dfman.agent"

func macNotify(ctx context.Context, args ...string) error {
	app, e := helperPath("dfman-notify.app")
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp("", "dfman-notify-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	result := filepath.Join(dir, "result.json")
	argv := []string{"-W", "-n", app, "--args"}
	argv = append(argv, args...)
	argv = append(argv, result)
	_, e = runCommand(ctx, "", nil, "/usr/bin/open", argv...)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(result)
	if e != nil {
		return fmt.Errorf("notification helper returned no result: %w", e)
	}
	var r struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return e
	}
	if !r.OK {
		return fmt.Errorf("notifications: %s", r.Error)
	}
	return nil
}
func desktopNotify(ctx context.Context, title, body, key string) error {
	return macNotify(ctx, "send", title, body, key)
}
func installAgent(ctx context.Context, exe, config, state string, c Config, out io.Writer) error {
	if c.Notification {
		if e := macNotify(ctx, "authorize"); e != nil {
			_ = reportProblem(state, "notification", e.Error())
		}
	}
	h, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	plist := filepath.Join(h, "Library", "LaunchAgents", agentLabel+".plist")
	d, _ := c.Interval()
	uid := strconv.Itoa(os.Getuid())
	domain := "gui/" + uid
	if _, e = runCommand(ctx, "", nil, "launchctl", "print", domain); e != nil {
		return fmt.Errorf("install from a signed-in desktop session: %w", e)
	}
	xml := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>` + agentLabel + `</string><key>ProgramArguments</key><array>`
	for _, arg := range []string{exe, "--config", config, "--state-dir", state, "agent", "run"} {
		xml += "<string>" + html.EscapeString(arg) + "</string>"
	}
	xml += fmt.Sprintf(`</array><key>StartInterval</key><integer>%d</integer><key>RunAtLoad</key><false/><key>LimitLoadToSessionType</key><string>Aqua</string><key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string></dict></dict></plist>`, int(d/time.Second), html.EscapeString(filepath.Join(h, ".local", "bin")+":/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"))
	if e = writeAtomic(plist, []byte(xml), 0644); e != nil {
		return e
	}
	_, _ = runCommand(ctx, "", nil, "launchctl", "bootout", domain+"/"+agentLabel)
	if _, e = runCommand(ctx, "", nil, "launchctl", "bootstrap", domain, plist); e != nil {
		return e
	}
	if _, e = runCommand(ctx, "", nil, "launchctl", "enable", domain+"/"+agentLabel); e != nil {
		return e
	}
	fmt.Fprintf(out, "Installed agent: every %s (%s).\n", d, c.Agent.SyncMode)
	return nil
}
func uninstallAgent(ctx context.Context, out io.Writer) error {
	h, _ := os.UserHomeDir()
	domain := "gui/" + strconv.Itoa(os.Getuid()) + "/" + agentLabel
	if _, e := runCommand(ctx, "", nil, "launchctl", "print", domain); e == nil {
		if _, e = runCommand(ctx, "", nil, "launchctl", "bootout", domain); e != nil {
			return e
		}
	}
	e := os.Remove(filepath.Join(h, "Library", "LaunchAgents", agentLabel+".plist"))
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	fmt.Fprintln(out, "Agent uninstalled.")
	return nil
}
func platformAgentStatus(ctx context.Context, out io.Writer) error {
	s, e := runCommand(ctx, "", nil, "launchctl", "print", "gui/"+strconv.Itoa(os.Getuid())+"/"+agentLabel)
	fmt.Fprintln(out, s)
	return e
}
