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

	if code := Execute([]string{"--config", config, "--state-dir", filepath.Join(d, "state"), "agent", "run"}, "dev", nil, io.Discard, io.Discard); code != 0 {
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

func TestPackageLoginHonorsDisabledAgent(t *testing.T) {
	d := t.TempDir()
	state := filepath.Join(d, "state")
	put(t, filepath.Join(state, "agent-disabled"), "disabled")
	var output strings.Builder
	if code := Execute([]string{"--config", filepath.Join(d, "missing"), "--state-dir", state, "_package", "login"}, "dev", nil, &output, io.Discard); code != 0 {
		t.Fatal(code)
	}
	if output.Len() != 0 {
		t.Fatal(output.String())
	}
}
