package dfman

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type release struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func download(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "dfman")
	client := &http.Client{Timeout: 2 * time.Minute}
	r, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("download: HTTP %d", r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("download exceeds size limit")
	}
	return b, e
}
func verifyChecksum(data, checksums []byte, name string) error {
	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			if expected != "" {
				return fmt.Errorf("duplicate checksum")
			}
			expected = fields[0]
		}
	}
	sum := sha256.Sum256(data)
	if expected == "" || !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
		return fmt.Errorf("checksum verification failed for %s", name)
	}
	return nil
}
func extractPackage(path, dest string) error {
	z, e := zip.OpenReader(path)
	if e != nil {
		return e
	}
	defer z.Close()
	var total uint64
	for _, f := range z.File {
		p := filepath.FromSlash(f.Name)
		if strings.HasPrefix(f.Name, "/") || filepath.IsAbs(p) || !within(dest, filepath.Join(dest, p)) || strings.Contains(p, ":") || strings.Contains(f.Name, `\`) {
			return fmt.Errorf("unsafe archive path: %s", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive symlinks are unsupported")
		}
		if f.UncompressedSize64 > 256<<20 {
			return fmt.Errorf("expanded file exceeds size limit")
		}
		total += f.UncompressedSize64
		if total > 256<<20 {
			return fmt.Errorf("expanded package exceeds size limit")
		}
		p = filepath.Join(dest, p)
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(p, 0755); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			return e
		}
		src, e := f.Open()
		if e != nil {
			return e
		}
		dst, e := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, f.Mode().Perm())
		if e != nil {
			src.Close()
			return e
		}
		_, e = io.Copy(dst, io.LimitReader(src, int64(f.UncompressedSize64)+1))
		ce := dst.Close()
		src.Close()
		if e != nil {
			return e
		}
		if ce != nil {
			return ce
		}
	}
	return nil
}
func latestRelease(ctx context.Context) (release, error) {
	var r release
	b, err := download(ctx, "https://api.github.com/repos/kopiro/dfman/releases/latest", 2<<20)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	if r.Tag == "" {
		return r, fmt.Errorf("release has no version")
	}
	return r, nil
}

func SelfUpdate(ctx context.Context, current string, check bool, out io.Writer) error {
	r, e := latestRelease(ctx)
	if e != nil {
		return e
	}
	if r.Tag == current {
		fmt.Fprintln(out, "Already up to date:", current)
		return nil
	}
	fmt.Fprintf(out, "Available: %s (installed: %s)\n", r.Tag, current)
	if check {
		return nil
	}
	if kind := strings.TrimSpace(packageKind()); kind != "" {
		return fmt.Errorf("installed with %s; download and run the latest installer from https://github.com/kopiro/dfman/releases/latest", kind)
	}
	name := fmt.Sprintf("dfman_%s_%s_%s.zip", r.Tag, runtime.GOOS, runtime.GOARCH)
	urls := map[string]string{}
	for _, a := range r.Assets {
		urls[a.Name] = a.URL
	}
	if urls[name] == "" || urls["checksums.txt"] == "" {
		return fmt.Errorf("release does not include this platform or checksums")
	}
	sums, e := download(ctx, urls["checksums.txt"], 1<<20)
	if e != nil {
		return e
	}
	data, e := download(ctx, urls[name], 128<<20)
	if e != nil {
		return e
	}
	if e = verifyChecksum(data, sums, name); e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	exe, e = filepath.EvalSymlinks(exe)
	if e != nil {
		return e
	}
	stage, e := os.MkdirTemp(filepath.Dir(exe), ".dfman-update-")
	if e != nil {
		return e
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(stage)
		}
	}()
	archive := filepath.Join(stage, "package.zip")
	if e = os.WriteFile(archive, data, 0600); e != nil {
		return e
	}
	payload := filepath.Join(stage, "payload")
	if e = extractPackage(archive, payload); e != nil {
		return e
	}
	if e = validatePackage(payload); e != nil {
		return e
	}
	keep, e = installUpdate(ctx, payload, filepath.Dir(exe), out)
	return e
}
func packageFiles() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"dfman", "dfman-notify.app"}
	case "windows":
		return []string{"dfman.exe", "dfman-notify.exe"}
	default:
		return []string{"dfman"}
	}
}
func validatePackage(dir string) error {
	for _, name := range packageFiles() {
		p := filepath.Join(dir, name)
		s, e := os.Stat(p)
		if e != nil {
			return e
		}
		if name == "dfman-notify.app" {
			if !s.IsDir() {
				return fmt.Errorf("missing notification app")
			}
			if _, e = os.Stat(filepath.Join(p, "Contents", "MacOS", "dfman-notify")); e != nil {
				return e
			}
		} else if !s.Mode().IsRegular() {
			return fmt.Errorf("invalid executable: %s", name)
		}
	}
	return nil
}
