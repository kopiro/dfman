package dfman

import (
	"context"
	"fmt"
	"github.com/godbus/dbus/v5"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func desktopNotify(ctx context.Context, title, body, key string) error {
	conn, e := dbus.ConnectSessionBus()
	if e != nil {
		return e
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var id uint32
	return conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0, "dfman", uint32(0), "", title, body, []string{}, map[string]dbus.Variant{}, int32(-1)).Store(&id)
}
func unitQuote(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return "\"" + s + "\""
}
func installAgent(ctx context.Context, exe, config, state string, c Config, out io.Writer) error {
	if _, e := runCommand(ctx, "", nil, "systemctl", "--user", "show-environment"); e != nil {
		return e
	}
	h, _ := os.UserHomeDir()
	dir := filepath.Join(h, ".config", "systemd", "user")
	d, _ := c.Interval()
	args := []string{exe, "--config", config, "--state-dir", state, "agent", "run"}
	for i := range args {
		args[i] = unitQuote(args[i])
	}
	service := "[Unit]\nDescription=dfman dotfile synchronization\nPartOf=graphical-session.target\nAfter=graphical-session.target\n\n[Service]\nType=oneshot\nExecStart=" + strings.Join(args, " ") + "\nTimeoutStartSec=10min\n"
	timer := fmt.Sprintf("[Unit]\nDescription=dfman scheduled synchronization\nPartOf=graphical-session.target\nRequisite=graphical-session.target\nAfter=graphical-session.target\n\n[Timer]\nOnActiveSec=%ds\nOnUnitInactiveSec=%ds\nAccuracySec=1s\nUnit=dfman-agent.service\n\n[Install]\nWantedBy=graphical-session.target\n", int(d/time.Second), int(d/time.Second))
	if e := writeAtomic(filepath.Join(dir, "dfman-agent.service"), []byte(service), 0644); e != nil {
		return e
	}
	if e := writeAtomic(filepath.Join(dir, "dfman-agent.timer"), []byte(timer), 0644); e != nil {
		return e
	}
	for _, a := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", "dfman-agent.timer"}, {"--user", "restart", "dfman-agent.timer"}} {
		if _, e := runCommand(ctx, "", nil, "systemctl", a...); e != nil {
			return e
		}
	}
	fmt.Fprintf(out, "Installed agent: every %s (%s).\n", d, c.Agent.SyncMode)
	return nil
}
func uninstallAgent(ctx context.Context, out io.Writer) error {
	h, _ := os.UserHomeDir()
	dir := filepath.Join(h, ".config", "systemd", "user")
	_, busErr := runCommand(ctx, "", nil, "systemctl", "--user", "show-environment")
	if busErr == nil {
		if _, e := os.Stat(filepath.Join(dir, "dfman-agent.timer")); e == nil {
			if _, e = runCommand(ctx, "", nil, "systemctl", "--user", "disable", "--now", "dfman-agent.timer"); e != nil {
				return e
			}
		}
		if _, e := runCommand(ctx, "", nil, "systemctl", "--user", "is-active", "dfman-agent.service"); e == nil {
			if _, e = runCommand(ctx, "", nil, "systemctl", "--user", "stop", "dfman-agent.service"); e != nil {
				return e
			}
		}
	}
	if e := os.Remove(filepath.Join(dir, "graphical-session.target.wants", "dfman-agent.timer")); e != nil && !os.IsNotExist(e) {
		return e
	}
	for _, n := range []string{"dfman-agent.timer", "dfman-agent.service"} {
		if e := os.Remove(filepath.Join(dir, n)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	if busErr == nil {
		if _, e := runCommand(ctx, "", nil, "systemctl", "--user", "daemon-reload"); e != nil {
			return e
		}
	}
	fmt.Fprintln(out, "Agent uninstalled.")
	return nil
}
func platformAgentStatus(ctx context.Context, out io.Writer) error {
	s, e := runCommand(ctx, "", nil, "systemctl", "--user", "status", "dfman-agent.timer", "--no-pager")
	fmt.Fprintln(out, s)
	return e
}
