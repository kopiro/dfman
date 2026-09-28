package dfman

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Folder struct {
	Source string `toml:"source"`
	Target string `toml:"target"`
}
type AgentConfig struct {
	Enabled  bool   `toml:"enabled"`
	Interval string `toml:"interval"`
	SyncMode string `toml:"sync_mode"`
}
type Config struct {
	Notification bool        `toml:"notification"`
	Agent        AgentConfig `toml:"agent"`
	Folders      []Folder    `toml:"folders"`
}

func DefaultConfig() Config {
	return Config{Notification: true, Agent: AgentConfig{Enabled: true, Interval: "10m", SyncMode: "normal"}}
}
func configPath() string { h, _ := os.UserHomeDir(); return filepath.Join(h, ".config", "dfman.conf") }
func statePath() string {
	h, _ := os.UserHomeDir()
	if s := os.Getenv("XDG_STATE_HOME"); s != "" {
		return filepath.Join(s, "dfman")
	}
	return filepath.Join(h, ".local", "state", "dfman")
}
func expand(p string) (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if p == "~" {
		p = h
	} else if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		p = filepath.Join(h, p[2:])
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be absolute or start with ~: %q", p)
	}
	return filepath.Clean(p), nil
}
func (c Config) Interval() (time.Duration, error) {
	d, e := time.ParseDuration(c.Agent.Interval)
	if e != nil || d < time.Minute || d > 24*time.Hour || d%time.Minute != 0 {
		return 0, fmt.Errorf("agent.interval must be whole minutes between 1m and 24h")
	}
	return d, nil
}
func LoadConfig(path string) (Config, error) {
	c := DefaultConfig()
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf}) // Windows editors may emit a UTF-8 BOM.
	dec := toml.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e = dec.Decode(&c); e != nil {
		return c, fmt.Errorf("%s: invalid TOML configuration (legacy line-based configs are unsupported): %w", path, e)
	}
	if _, e = c.Interval(); e != nil {
		return c, e
	}
	if c.Agent.SyncMode != "normal" && c.Agent.SyncMode != "reset" {
		return c, fmt.Errorf("agent.sync_mode must be normal or reset")
	}
	for i := range c.Folders {
		f := &c.Folders[i]
		if f.Target == "" {
			f.Target = "~"
		}
		if _, e = expand(f.Source); e != nil {
			return c, fmt.Errorf("folders[%d].source: %w", i, e)
		}
		if _, e = expand(f.Target); e != nil {
			return c, fmt.Errorf("folders[%d].target: %w", i, e)
		}
	}
	return c, nil
}
func (c Config) Selected(repo string) ([]Folder, error) {
	var out []Folder
	var wanted string
	var err error
	if repo != "" {
		wanted, err = expand(repo)
		if err != nil {
			return nil, err
		}
	}
	for _, f := range c.Folders {
		f.Source, _ = expand(f.Source)
		f.Target, _ = expand(f.Target)
		info, e := os.Stat(f.Source)
		if e != nil {
			return nil, fmt.Errorf("source %s: %w", f.Source, e)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source is not a directory: %s", f.Source)
		}
		if info, e = os.Stat(f.Target); e == nil {
			if !info.IsDir() {
				return nil, fmt.Errorf("target is not a directory: %s", f.Target)
			}
		} else if !os.IsNotExist(e) {
			return nil, fmt.Errorf("target %s: %w", f.Target, e)
		}
		if wanted == "" || samePath(wanted, f.Source) {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no configured folders match %q", repo)
	}
	return out, nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	// Preserve managed configuration symlinks by replacing their authoritative target.
	if p, e := filepath.EvalSymlinks(path); e == nil {
		path = p
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".dfman-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return replaceFile(tmp, path)
}
func SaveConfig(path string, c Config) error {
	b, e := toml.Marshal(c)
	if e != nil {
		return e
	}
	return writeAtomic(path, b, 0600)
}
