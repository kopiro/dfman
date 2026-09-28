package dfman

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestAgentUpdateNotices(t *testing.T) {
	state := t.TempDir()
	now := time.Now()
	checks, notices := 0, 0
	tag := "v1.1.0"
	failure := false
	fetch := func(context.Context) (release, error) {
		checks++
		if failure {
			return release{}, errors.New("offline")
		}
		return release{Tag: tag}, nil
	}
	notify := func(context.Context, string, string, string) error { notices++; return nil }
	run := func() error {
		return agentUpdateNotice(context.Background(), "v1.0.0", state, io.Discard, notify, fetch, now)
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if checks != 1 || notices != 1 {
		t.Fatalf("checks=%d notices=%d", checks, notices)
	}
	now = now.Add(24 * time.Hour)
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if checks != 2 || notices != 1 {
		t.Fatal("repeated version notified")
	}
	tag = "v1.2.0"
	now = now.Add(24 * time.Hour)
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if notices != 2 {
		t.Fatal("new version not notified")
	}
	failure = true
	now = now.Add(24 * time.Hour)
	if err := run(); err == nil {
		t.Fatal("failure hidden")
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	if checks != 4 || notices != 2 {
		t.Fatal("offline check not throttled")
	}
}

func TestNewerRelease(t *testing.T) {
	for _, tc := range []struct {
		candidate, current string
		want               bool
	}{
		{"v1.1.0", "v1.0.0", true}, {"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v2.0.0", false}, {"v1.0.0", "dev", false},
		{"v1.1.0-rc1", "v1.0.0", false},
	} {
		if newerRelease(tc.candidate, tc.current) != tc.want {
			t.Fatal(tc)
		}
	}
}

func TestShellCommandRemoved(t *testing.T) {
	if Execute([]string{"shell", "init"}, "dev", nil, io.Discard, io.Discard) == 0 {
		t.Fatal("shell command still supported")
	}
}
