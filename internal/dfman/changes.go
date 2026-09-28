package dfman

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func changedFiles(ctx context.Context, repo, before, after string) ([]FileChange, error) {
	raw, err := git(ctx, repo, "", "diff", "--no-ext-diff", "--no-textconv", "--name-status", "-z", "--find-renames", before, after, "--")
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "\x00")
	if parts[len(parts)-1] != "" {
		return nil, fmt.Errorf("incomplete file change list")
	}
	parts = parts[:len(parts)-1]
	var changes []FileChange
	for len(parts) > 0 {
		if len(parts) < 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid file change list")
		}
		status, path := parts[0], parts[1]
		parts = parts[2:]
		change := FileChange{Path: path}
		switch status[0] {
		case 'A':
			change.Kind = "Added"
		case 'D':
			change.Kind = "Deleted"
		case 'M', 'T':
			change.Kind = "Modified"
		case 'R', 'C':
			if len(parts) == 0 {
				return nil, fmt.Errorf("incomplete rename")
			}
			change.Kind = "Renamed"
			if status[0] == 'C' {
				change.Kind = "Copied"
			}
			change.OldPath, change.Path = path, parts[0]
			parts = parts[1:]
		default:
			return nil, fmt.Errorf("unsupported file change: %s", status)
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// Quote unusual filenames so line breaks and control characters cannot masquerade
// as additional changes in notifications or terminal output.
func displayPath(path string) string {
	for _, r := range path {
		if r < 32 || r == 127 {
			return strconv.Quote(path)
		}
	}
	return path
}
func transferLines(r Result) []string {
	var lines []string
	for _, direction := range []struct {
		label string
		files []FileChange
	}{
		{"Pulled", r.PulledFiles}, {"Pushed", r.PushedFiles},
	} {
		for _, f := range direction.files {
			path := displayPath(f.Path)
			if f.OldPath != "" {
				path = displayPath(f.OldPath) + " → " + path
			}
			lines = append(lines, direction.label+" · "+f.Kind+": "+path)
		}
	}
	return lines
}
func transferNotice(r Result) (string, string) {
	lines := transferLines(r)
	if len(lines) == 0 {
		return "", ""
	}
	title := "Pulled updates"
	if len(r.PulledFiles) == 0 {
		title = "Pushed updates"
	} else if len(r.PushedFiles) > 0 {
		title = "Sync complete"
	}
	total := len(lines)
	if total > 3 {
		lines = lines[:3]
	}
	for i, line := range lines {
		chars := []rune(line)
		if len(chars) > 100 {
			lines[i] = string(chars[:99]) + "…"
		}
	}
	if total > 3 {
		lines = append(lines, fmt.Sprintf("+%d more. Run dfman status for details.", total-3))
	}
	return title, strings.Join(lines, "\n")
}
