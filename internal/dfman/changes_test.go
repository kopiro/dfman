package dfman

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryOnlyTransfersStaySilent(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "reset"}[reset], func(t *testing.T) {
			f := setup(t)
			c := DefaultConfig()
			c.Folders = f.folders()
			if reset {
				c.Agent.SyncMode = "reset"
			}
			calls := 0
			notify := func(context.Context, string, string, string) error { calls++; return nil }
			g(t, f.other, "commit", "--allow-empty", "-m", "history only")
			g(t, f.other, "push")
			if err := runAgent(context.Background(), c, filepath.Join(f.dir, "state"), io.Discard, notify, ""); err != nil {
				t.Fatal(err)
			}
			if !reset {
				g(t, f.local, "commit", "--allow-empty", "-m", "local history only")
				if err := runAgent(context.Background(), c, filepath.Join(f.dir, "state"), io.Discard, notify, ""); err != nil {
					t.Fatal(err)
				}
			}
			if calls != 0 {
				t.Fatalf("history-only notifications: %d", calls)
			}
		})
	}
}

func TestTransferredFileLists(t *testing.T) {
	f := setup(t)
	// Establish files that will be deleted and renamed.
	put(t, filepath.Join(f.other, "dotfiles", "delete me"), "delete\n")
	put(t, filepath.Join(f.other, "dotfiles", "old name"), "unique rename contents\n")
	g(t, f.other, "add", ".")
	g(t, f.other, "commit", "-m", "prepare")
	g(t, f.other, "push")
	results := Sync(context.Background(), f.folders(), false, "")
	if resultCode(results) != 0 {
		t.Fatal(results)
	}
	put(t, filepath.Join(f.other, "dotfiles", ".test"), "modified\n")
	put(t, filepath.Join(f.other, "dotfiles", "new file"), "added\n")
	if err := os.Remove(filepath.Join(f.other, "dotfiles", "delete me")); err != nil {
		t.Fatal(err)
	}
	g(t, f.other, "mv", "dotfiles/old name", "dotfiles/new name")
	g(t, f.other, "add", ".")
	g(t, f.other, "commit", "-m", "file changes")
	g(t, f.other, "push")
	c := DefaultConfig()
	c.Folders = f.folders()
	state := filepath.Join(f.dir, "state")
	var body string
	calls := 0
	notify := func(_ context.Context, _ string, b string, _ string) error { calls++; body = b; return nil }
	if err := runAgent(context.Background(), c, state, io.Discard, notify, ""); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(body, "+1 more") {
		t.Fatalf("%d %s", calls, body)
	}
	summary, _, err := statusText(state, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Modified: dotfiles/.test", "Deleted: dotfiles/delete me", "Added: dotfiles/new file", "Renamed: dotfiles/old name → dotfiles/new name"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %s", want, summary)
		}
	}
	put(t, filepath.Join(f.local, "dotfiles", "outbound"), "push\n")
	if err := runAgent(context.Background(), c, state, io.Discard, notify, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Pushed · Added: dotfiles/outbound") {
		t.Fatal(body)
	}
	if err := runAgent(context.Background(), c, state, io.Discard, notify, ""); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("unchanged run notified: %d", calls)
	}
}

func TestFileChangeNamesRemainIntact(t *testing.T) {
	f := setup(t)
	before := g(t, f.local, "rev-parse", "HEAD")
	name := " spaces and tabs\t "
	if filepath.Separator == '\\' {
		name = " spaces inside" // Windows normalizes trailing spaces in filenames.
	}
	put(t, filepath.Join(f.local, name), "contents")
	g(t, f.local, "add", ".")
	g(t, f.local, "commit", "-m", "unusual name")
	changes, err := changedFiles(context.Background(), f.local, before, "HEAD")
	if err != nil || len(changes) != 1 || changes[0].Path != name {
		t.Fatalf("%+v %v", changes, err)
	}
}
