package dfman

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const shellInit = `_dfman_prompt_notice() {
    local previous_rc=$? notice
    [[ -t 1 ]] || return "$previous_rc"
    (( $+commands[dfman] )) || return "$previous_rc"
    notice=$(command dfman shell status 2>/dev/null) || return "$previous_rc"
    if [[ "$notice" != "${_dfman_seen_notice:-}" ]]; then
        [[ -z "$notice" ]] || print -r -- "$notice"
        _dfman_seen_notice=$notice
    fi
    return "$previous_rc"
}
autoload -Uz add-zsh-hook
add-zsh-hook -d precmd _dfman_prompt_notice 2>/dev/null
add-zsh-hook precmd _dfman_prompt_notice
`

func shellInstall(action string, out io.Writer) error {
	h, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	if z := os.Getenv("ZDOTDIR"); z != "" {
		h = z
	}
	path := filepath.Join(h, ".zshrc")
	b, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	var keep []string
	inside := false
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		switch line {
		case "# >>> dfman shell >>>":
			if inside {
				return fmt.Errorf("duplicate dfman shell marker")
			}
			inside = true
		case "# <<< dfman shell <<<":
			if !inside {
				return fmt.Errorf("unmatched dfman shell marker")
			}
			inside = false
		default:
			if !inside {
				keep = append(keep, line)
			}
		}
	}
	if inside {
		return fmt.Errorf("unterminated dfman shell marker")
	}
	if action == "install" {
		keep = append(keep, "# >>> dfman shell >>>", `if command -v dfman >/dev/null 2>&1; then`, `  eval "$(dfman shell init zsh)"`, "fi", "# <<< dfman shell <<<")
	}
	mode := os.FileMode(0644)
	if info, e := os.Stat(path); e == nil {
		mode = info.Mode().Perm()
	}
	if e = writeAtomic(path, []byte(strings.Join(keep, "\n")+"\n"), mode); e != nil {
		return e
	}
	fmt.Fprintf(out, "Shell integration %sed: %s\n", action, path)
	return nil
}
