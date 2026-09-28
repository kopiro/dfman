//go:build !windows

package dfman

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func installUpdate(ctx context.Context, payload, dest string, out io.Writer) (bool, error) {
	// Keep previous components beside the staged package until every rename succeeds.
	backup := filepath.Join(filepath.Dir(payload), "previous")
	if e := os.Mkdir(backup, 0700); e != nil {
		return false, e
	}
	var moved, installed []string
	rollback := func() {
		for _, n := range installed {
			os.RemoveAll(filepath.Join(dest, n))
		}
		for _, n := range moved {
			os.Rename(filepath.Join(backup, n), filepath.Join(dest, n))
		}
	}
	for _, n := range packageFiles() {
		if _, e := os.Lstat(filepath.Join(dest, n)); e == nil {
			if e = os.Rename(filepath.Join(dest, n), filepath.Join(backup, n)); e != nil {
				rollback()
				return false, e
			}
			moved = append(moved, n)
		}
		if e := os.Rename(filepath.Join(payload, n), filepath.Join(dest, n)); e != nil {
			rollback()
			return false, e
		}
		installed = append(installed, n)
	}
	fmt.Fprintln(out, "Update installed. Run dfman agent install if you use the agent.")
	return false, nil
}
