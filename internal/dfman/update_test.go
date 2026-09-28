package dfman

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestUpdateReplacement(t *testing.T) {
	if os.Getenv("DFMAN_UPDATE_CHILD") == "1" {
		_, e := installUpdate(context.Background(), os.Getenv("DFMAN_UPDATE_PAYLOAD"), os.Getenv("DFMAN_UPDATE_DEST"), io.Discard)
		if e != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	d := t.TempDir()
	payload, dest := filepath.Join(d, "stage", "payload"), filepath.Join(d, "bin")
	for _, n := range packageFiles() {
		file := n
		if n == "dfman-notify.app" {
			file = filepath.Join(n, "Contents", "MacOS", "dfman-notify")
		}
		put(t, filepath.Join(payload, file), "new")
		put(t, filepath.Join(dest, file), "old")
	}
	if runtime.GOOS == "windows" {
		c := exec.Command(testExecutable(t), "-test.run=^TestUpdateReplacement$")
		c.Env = append(os.Environ(), "DFMAN_UPDATE_CHILD=1", "DFMAN_UPDATE_PAYLOAD="+payload, "DFMAN_UPDATE_DEST="+dest)
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("updater child: %v %s", e, b)
		}
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			b, _ := os.ReadFile(filepath.Join(dest, "dfman-update.log"))
			if string(b) != "" {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	} else {
		if _, e := installUpdate(context.Background(), payload, dest, io.Discard); e != nil {
			t.Fatal(e)
		}
	}
	for _, n := range packageFiles() {
		if n == "dfman-notify.app" {
			n = filepath.Join(n, "Contents", "MacOS", "dfman-notify")
		}
		if got := get(t, filepath.Join(dest, n)); got != "new" {
			t.Fatalf("replacement %s: %s", n, got)
		}
	}
}
