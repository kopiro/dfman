package dfman

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type release struct{ Tag string }

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
		return nil, fmt.Errorf("download from %s: HTTP %d; check network access or retry later", req.URL.Host, r.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("download exceeds size limit")
	}
	return b, e
}
func latestRelease(ctx context.Context) (release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://github.com/kopiro/dfman/releases/latest", nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("User-Agent", "dfman")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("release check at github.com: HTTP %d; check network access or retry later", resp.StatusCode)
	}
	return releaseFromURL(resp.Request.URL)
}

func releaseFromURL(u *url.URL) (release, error) {
	const prefix = "/kopiro/dfman/releases/tag/"
	if u.Scheme != "https" || u.Host != "github.com" || !strings.HasPrefix(u.Path, prefix) {
		return release{}, fmt.Errorf("unexpected release redirect")
	}
	tag := strings.TrimPrefix(u.Path, prefix)
	if _, ok := releaseVersion(tag); !ok || !strings.HasPrefix(tag, "v") {
		return release{}, fmt.Errorf("invalid release version")
	}
	return release{Tag: tag}, nil
}

func installerName(tag, platform, arch string) (string, error) {
	suffix := ""
	switch platform {
	case "darwin":
		suffix = ".pkg"
	case "linux":
		suffix = ".deb"
	case "windows":
		suffix = "_setup.exe"
	default:
		return "", fmt.Errorf("unsupported platform: %s", platform)
	}
	if arch != "amd64" && arch != "arm64" {
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
	return fmt.Sprintf("dfman_%s_%s_%s%s", tag, platform, arch, suffix), nil
}
func verifyInstaller(data, manifest []byte, name string) error {
	expected := ""
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			if expected != "" {
				return fmt.Errorf("duplicate installer checksum")
			}
			expected = fields[0]
		}
	}
	sum := sha256.Sum256(data)
	if expected == "" || !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
		return fmt.Errorf("installer checksum verification failed")
	}
	return nil
}
func launchInstaller(ctx context.Context, path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "/usr/bin/open", path).Run()
	case "windows":
		script := "Start-Process -FilePath '" + strings.ReplaceAll(path, "'", "''") + "'"
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Run()
	case "linux":
		cmd := exec.CommandContext(ctx, "pkexec", "/usr/bin/apt-get", "install", "-y", path)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("unsupported installer platform")
}

func SelfUpdate(ctx context.Context, current string, check bool, out io.Writer) error {
	r, e := latestRelease(ctx)
	if e != nil {
		return e
	}
	_, currentIsRelease := releaseVersion(current)
	if r.Tag == current || (currentIsRelease && !newerRelease(r.Tag, current)) {
		fmt.Fprintln(out, "Already up to date:", current)
		return nil
	}
	fmt.Fprintf(out, "Available: %s (installed: %s)\n", r.Tag, current)
	if check {
		return nil
	}
	name, err := installerName(r.Tag, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	base := "https://github.com/kopiro/dfman/releases/download/" + r.Tag + "/"
	manifest, err := download(ctx, base+"checksums.txt", 1<<20)
	if err != nil {
		return err
	}
	data, err := download(ctx, base+name, 128<<20)
	if err != nil {
		return err
	}
	if err = verifyInstaller(data, manifest, name); err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(cache, "dfman", "updates", r.Tag)
	path := filepath.Join(dir, name)
	if err = writeAtomic(path, data, 0700); err != nil {
		return err
	}
	fmt.Fprintln(out, "Verified installer:", path)
	if err = launchInstaller(ctx, path); err != nil {
		return fmt.Errorf("could not launch installer; run %s manually: %w", path, err)
	}
	if runtime.GOOS == "linux" {
		fmt.Fprintln(out, "Update installed.")
	} else {
		fmt.Fprintln(out, "Installer opened. Complete installation in its window.")
	}
	return nil
}
