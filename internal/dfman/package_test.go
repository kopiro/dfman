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
	put(t, config, "invalid config\n")
	if code := Execute([]string{"--config", config, "--state-dir", filepath.Join(d, "state"), "_package", "setup"}, "dev", nil, io.Discard, io.Discard); code == 0 {
		t.Fatal("accepted invalid config")
	}
	b, err := os.ReadFile(config)
	if err != nil || string(b) != "invalid config\n" {
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
