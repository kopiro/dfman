package dfman

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemovedAgentInstall(t *testing.T) {
	if Execute([]string{"agent", "install"}, "dev", nil, io.Discard, io.Discard) == 0 {
		t.Fatal("agent install is still accepted")
	}
}

func TestEmptyAgentConfigIsIdle(t *testing.T) {
	d := t.TempDir()
	config := filepath.Join(d, "config")
	put(t, config, "notification=false\n")
	if code := Execute([]string{"--config", config, "config", "validate"}, "dev", nil, io.Discard, io.Discard); code != 0 {
		t.Fatal("empty config is invalid", code)
	}

	if code := Execute([]string{"--config", config, "--state-dir", filepath.Join(d, "state"), "_package", "run"}, "dev", nil, io.Discard, io.Discard); code != 0 {
		t.Fatal(code)
	}
}

func TestPackageSetupPreservesInvalidConfig(t *testing.T) {
	d := t.TempDir()
	config := filepath.Join(d, "config")
	put(t, config, "old config\n")
	if code := Execute([]string{"--config", config, "--state-dir", filepath.Join(d, "state"), "_package", "setup"}, "dev", nil, io.Discard, io.Discard); code == 0 {
		t.Fatal("accepted invalid config")
	}
	b, err := os.ReadFile(config)
	if err != nil || string(b) != "old config\n" {
		t.Fatal("config overwritten", err)
	}
}

func TestDisabledAgentDoesNoWork(t *testing.T) {
	d := t.TempDir()
	config := filepath.Join(d, "config")
	put(t, config, "notification=true\n[agent]\nenabled=false\n[[folders]]\nsource='~/missing-folder'\n")
	var output strings.Builder
	if code := Execute([]string{"--config", config, "--state-dir", filepath.Join(d, "state"), "_package", "run"}, "v1.0.0", nil, &output, io.Discard); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(output.String(), "disabled") {
		t.Fatal(output.String())
	}
	if _, err := os.Stat(filepath.Join(d, "state")); !os.IsNotExist(err) {
		t.Fatal("disabled run created state", err)
	}
}
func TestAgentGroupRemoved(t *testing.T) {
	for _, args := range [][]string{{"agent"}, {"agent", "run"}, {"agent", "status"}, {"agent", "uninstall"}} {
		if Execute(args, "dev", nil, io.Discard, io.Discard) == 0 {
			t.Fatal(args)
		}
	}
}

func TestMigrateLegacyCommand(t *testing.T) {
	for _, kind := range []string{"script", "symlink", "dangling", "directory"} {
		t.Run(kind, func(t *testing.T) {
			d := t.TempDir()
			exe := filepath.Join(d, "packaged")
			old := filepath.Join(d, "dfman")
			state := filepath.Join(d, "state")
			put(t, exe, "new")
			switch kind {
			case "script":
				put(t, old, "#!/bin/bash\necho old\n")
			case "directory":
				if err := os.Mkdir(old, 0700); err != nil {
					t.Fatal(err)
				}
			default:
				target := filepath.Join(d, "previous")
				if kind == "symlink" {
					put(t, target, "old")
				}
				if err := os.Symlink(target, old); err != nil {
					t.Skip(err)
				}
			}
			err := migrateCommandPath(exe, old, state)
			if kind == "directory" {
				if err == nil {
					t.Fatal("moved directory")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if target, err := os.Readlink(old); err != nil || target != exe {
				t.Fatal(target, err)
			}
			backups, _ := filepath.Glob(filepath.Join(state, "package-backup", "*", "dfman"))
			if len(backups) != 1 {
				t.Fatal(backups)
			}
			if kind == "script" {
				b, err := os.ReadFile(backups[0])
				if err != nil || string(b) != "#!/bin/bash\necho old\n" {
					t.Fatal("lost backup", err)
				}
			} else {
				target, err := os.Readlink(backups[0])
				if err != nil || target != filepath.Join(d, "previous") {
					t.Fatal("lost symlink", err)
				}
			}
			if err := migrateCommandPath(exe, old, state); err != nil {
				t.Fatal(err)
			}
			again, _ := filepath.Glob(filepath.Join(state, "package-backup", "*", "dfman"))
			if len(again) != 1 {
				t.Fatal("reinstall created extra backup")
			}
		})
	}
}
