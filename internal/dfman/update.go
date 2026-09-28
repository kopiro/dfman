package dfman

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	fmt.Fprintln(out, "Download and run the installer: https://github.com/kopiro/dfman/releases/latest")
	return nil
}
