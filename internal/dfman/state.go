package dfman

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return writeAtomic(path, b, 0600)
}
func recordResults(state string, results []Result) error {
	for _, r := range results {
		if r.Kind == "busy" {
			continue
		}
		if r.Kind == "ok" {
			os.Remove(filepath.Join(state, "notifications", hash(r.Repo)))
		}
		if e := writeJSON(filepath.Join(state, "reports", hash(r.Repo)+".json"), r); e != nil {
			return e
		}
		// Import durable notices left by an interrupted reset, too.
		if dir, e := git(context.Background(), r.Repo, "", "rev-parse", "--absolute-git-dir"); e == nil {
			paths, _ := filepath.Glob(filepath.Join(filepath.FromSlash(dir), "dfman-recovery", "*", "notice.json"))
			for _, p := range paths {
				b, e := os.ReadFile(p)
				if e != nil {
					return e
				}
				var notice Result
				if e = json.Unmarshal(b, &notice); e != nil {
					return e
				}
				key := hash(notice.Backup) + ".json"
				if _, e = os.Stat(filepath.Join(state, "acknowledged", key)); os.IsNotExist(e) {
					if e = writeJSON(filepath.Join(state, "notices", key), notice); e != nil {
						return e
					}
				}
			}
		}
		if r.LocalSaved && r.Backup != "" {
			if e := writeJSON(filepath.Join(state, "notices", hash(r.Backup)+".json"), r); e != nil {
				return e
			}
		}
	}
	return writeAtomic(filepath.Join(state, "last-run"), []byte(time.Now().UTC().Format(time.RFC3339)), 0600)
}
func reportProblem(state, key, detail string) error {
	p := filepath.Join(state, "reports", key+".json")
	if detail == "" {
		os.Remove(filepath.Join(state, "notifications", hash(key)))
		e := os.Remove(p)
		if os.IsNotExist(e) {
			return nil
		}
		return e
	}
	return writeJSON(p, Result{Repo: key, Kind: "error", Detail: detail, At: time.Now()})
}
func statusText(state string, ack bool) (string, string, error) {
	if ack {
		files, _ := filepath.Glob(filepath.Join(state, "notices", "*.json"))
		for _, p := range files {
			if e := writeAtomic(filepath.Join(state, "acknowledged", filepath.Base(p)), []byte("acknowledged"), 0600); e != nil {
				return "", "", e
			}
			if e := os.Remove(p); e != nil {
				return "", "", e
			}
		}
	}
	var summary, problems strings.Builder
	for _, kind := range []string{"reports", "notices"} {
		files, _ := filepath.Glob(filepath.Join(state, kind, "*.json"))
		for _, p := range files {
			b, e := os.ReadFile(p)
			if e != nil {
				return "", "", e
			}
			var r Result
			if e = json.Unmarshal(b, &r); e != nil {
				return "", "", e
			}
			line := fmt.Sprintf("%s: %s\n%s\n", r.Repo, r.Kind, r.Detail)
			for _, entry := range transferLines(r) {
				line += entry + "\n"
			}
			if kind == "notices" {
				line = "Local differences saved: " + r.Repo + "\nRecovery: " + r.Backup + "\n"
			}
			summary.WriteString(line)
			if kind == "notices" || (r.Kind != "ok" && r.Kind != "busy") {
				problems.WriteString(line)
			}
		}
	}
	return summary.String(), problems.String(), nil
}
func printStatus(state string, ack bool, out io.Writer) error {
	s, _, e := statusText(state, ack)
	if e != nil {
		return e
	}
	last, _ := os.ReadFile(filepath.Join(state, "last-run"))
	fmt.Fprintf(out, "Last sync: %s\n", last)
	if s == "" {
		s = "No results recorded.\n"
	}
	fmt.Fprint(out, s)
	return nil
}
