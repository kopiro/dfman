package dfman

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
func within(base, path string) bool {
	r, e := filepath.Rel(base, path)
	return e == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

type LinkOptions struct {
	DryRun, Force, Verbose bool
	Input                  io.Reader
	Output                 io.Writer
}
type LinkEntry struct {
	Source, Target string
	Info           fs.FileInfo
}

func linkEntries(folders []Folder) ([]LinkEntry, error) {
	var all []LinkEntry
	seen := map[string]bool{}
	for _, f := range folders {
		entries, e := os.ReadDir(f.Source)
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			source := filepath.Join(f.Source, entry.Name())
			target := filepath.Join(f.Target, strings.ReplaceAll(entry.Name(), "#", string(filepath.Separator)))
			if !within(f.Target, target) || samePath(f.Target, target) {
				return nil, fmt.Errorf("unsafe link destination: %s", target)
			}
			key := target
			if runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			info, e := os.Lstat(source)
			if e != nil {
				return nil, e
			}
			all = append(all, LinkEntry{source, target, info})
		}
	}
	return all, nil
}
func Link(folders []Folder, opt LinkOptions) error {
	entries, e := linkEntries(folders)
	if e != nil {
		return e
	}
	var failures []error
	reader := bufio.NewReader(opt.Input)
	for _, entry := range entries {
		info, e := os.Lstat(entry.Target)
		if e == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				p, err := os.Readlink(entry.Target)
				if err != nil {
					failures = append(failures, err)
					continue
				}
				if !filepath.IsAbs(p) {
					p = filepath.Join(filepath.Dir(entry.Target), p)
				}
				if samePath(p, entry.Source) {
					if opt.Verbose {
						fmt.Fprintf(opt.Output, "OK: %s -> %s\n", entry.Target, entry.Source)
					}
					continue
				}
			}
			// Never remove directories, files, or links without an explicit interactive confirmation.
			if !opt.Force {
				fmt.Fprintf(opt.Output, "Skipped existing path: %s\n", entry.Target)
				continue
			}
			if !opt.DryRun {
				fmt.Fprintf(opt.Output, "Replace %s? [y/N] ", entry.Target)
				answer, _ := reader.ReadString('\n')
				if !strings.EqualFold(strings.TrimSpace(answer), "y") {
					continue
				}
				if e = os.RemoveAll(entry.Target); e != nil {
					failures = append(failures, e)
					continue
				}
			}
		} else if !os.IsNotExist(e) {
			failures = append(failures, e)
			continue
		}
		if opt.DryRun {
			fmt.Fprintf(opt.Output, "Would link: %s -> %s\n", entry.Target, entry.Source)
			continue
		}
		if e = os.MkdirAll(filepath.Dir(entry.Target), 0755); e == nil {
			e = os.Symlink(entry.Source, entry.Target)
		}
		if e != nil {
			failures = append(failures, fmt.Errorf("link %s: %w (Windows requires Developer Mode or symlink privilege)", entry.Target, e))
			continue
		}
		fmt.Fprintf(opt.Output, "Linked: %s -> %s\n", entry.Target, entry.Source)
	}
	return errors.Join(failures...)
}
func List(folders []Folder, w io.Writer) error {
	color := false
	if f, ok := w.(*os.File); ok && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		if info, e := f.Stat(); e == nil {
			color = info.Mode()&os.ModeCharDevice != 0
		}
	}
	heading, reset := "", ""
	if color {
		heading = "\x1b[1;36m"
		reset = "\x1b[0m"
	}
	for _, f := range folders {
		fmt.Fprintf(w, "%sRepository: %s%s\n  Target: %s\n", heading, f.Source, reset, f.Target)
		entries, e := os.ReadDir(f.Source)
		if e != nil {
			return e
		}
		for _, item := range entries {
			if item.Name() == ".git" {
				continue
			}
			c := ""
			if color {
				c = "\x1b[32m"
				if item.Type()&os.ModeSymlink != 0 {
					c = "\x1b[35m"
				} else if item.IsDir() {
					c = "\x1b[34m"
				}
			}
			fmt.Fprintf(w, "  %s%q%s -> %q\n", c, item.Name(), reset, filepath.Join(f.Target, strings.ReplaceAll(item.Name(), "#", string(filepath.Separator))))
		}
	}
	return nil
}
func Doctor(folders []Folder, w io.Writer) error {
	var failures []error
	for _, f := range folders {
		e := filepath.WalkDir(f.Source, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.Name() == ".git" && d.IsDir() {
				return filepath.SkipDir
			}
			if d.Type()&os.ModeSymlink != 0 {
				if _, e := os.Stat(p); e != nil {
					fmt.Fprintln(w, p)
					failures = append(failures, fmt.Errorf("broken symlink: %s", p))
				}
			}
			return nil
		})
		if e != nil {
			failures = append(failures, e)
		}
	}
	return errors.Join(failures...)
}
func copyPath(src, dst string) error {
	info, e := os.Lstat(src)
	if e != nil {
		return e
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, e := os.Readlink(src)
		if e != nil {
			return e
		}
		return os.Symlink(target, dst)
	}
	if info.IsDir() {
		if e = os.Mkdir(dst, info.Mode().Perm()); e != nil {
			return e
		}
		items, e := os.ReadDir(src)
		if e != nil {
			return e
		}
		for _, item := range items {
			if e = copyPath(filepath.Join(src, item.Name()), filepath.Join(dst, item.Name())); e != nil {
				return e
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported file: %s", src)
	}
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}
func Create(f Folder, path string, in io.Reader, out io.Writer) error {
	p, e := expand(path)
	if e != nil {
		return e
	}
	if !within(f.Target, p) || samePath(f.Target, p) {
		return fmt.Errorf("path must be beneath target %s", f.Target)
	}
	rel, _ := filepath.Rel(f.Target, p)
	dst := filepath.Join(f.Source, strings.ReplaceAll(rel, string(filepath.Separator), "#"))
	if _, e = os.Lstat(dst); !os.IsNotExist(e) {
		return fmt.Errorf("destination already exists: %s", dst)
	}
	if e = copyPath(p, dst); e != nil {
		return e
	}
	fmt.Fprintf(out, "Copied %s -> %s\nRemove original? [y/N] ", p, dst)
	answer, _ := bufio.NewReader(in).ReadString('\n')
	if strings.EqualFold(strings.TrimSpace(answer), "y") {
		return os.RemoveAll(p)
	}
	return nil
}
