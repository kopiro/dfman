//go:build !windows

package dfman

import "os"

func replaceFile(src, dst string) error { return os.Rename(src, dst) }
