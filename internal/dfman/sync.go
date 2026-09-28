package dfman

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

type FileChange struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
}

type Result struct {
	PulledFiles []FileChange `json:"pulled_files,omitempty"`
	PushedFiles []FileChange `json:"pushed_files,omitempty"`
	Repo        string       `json:"repo"`
	Kind        string       `json:"kind"`
	Detail      string       `json:"detail"`
	Pulled      bool         `json:"pulled,omitempty"`
	Pushed      bool         `json:"pushed,omitempty"`
	Backup      string       `json:"backup,omitempty"`
	LocalSaved  bool         `json:"local_saved,omitempty"`
	At          time.Time    `json:"at"`
}

func (r Result) Code() int {
	switch r.Kind {
	case "ok":
		return 0
	case "conflict":
		return 2
	case "busy":
		return 75
	default:
		return 1
	}
}
func resultCode(results []Result) int {
	code := 0
	for _, r := range results {
		switch r.Code() {
		case 2:
			code = 2
		case 1:
			if code != 2 {
				code = 1
			}
		case 75:
			if code == 0 {
				code = 75
			}
		}
	}
	return code
}
func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	configureCommand(cmd)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = 5 * time.Second
	b, e := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(b), fmt.Errorf("%s timed out: %w", name, ctx.Err())
	}
	if e != nil {
		return string(b), fmt.Errorf("%s: %w: %s", name, e, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}
func git(ctx context.Context, repo string, key string, args ...string) (string, error) {
	env := []string{"GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true", "GIT_MERGE_AUTOEDIT=no"}
	if key != "" {
		env = append(env, "GIT_SSH_COMMAND=ssh -i "+shellQuote(key)+" -o IdentitiesOnly=yes")
	}
	return runCommand(ctx, repo, env, "git", args...)
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func gitRoot(ctx context.Context, source string) (string, error) {
	p, e := git(ctx, source, "", "rev-parse", "--show-toplevel")
	if e != nil {
		return "", e
	}
	p = filepath.FromSlash(p)
	return filepath.EvalSymlinks(p)
}
func ancestor(ctx context.Context, repo, a, b string) (bool, error) {
	_, e := git(ctx, repo, "", "merge-base", "--is-ancestor", a, b)
	if e == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(e, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, e
}
func Sync(ctx context.Context, folders []Folder, reset bool, key string) []Result {
	roots := []string{}
	seen := map[string]bool{}
	var results []Result
	for _, f := range folders {
		root, e := gitRoot(ctx, f.Source)
		if e != nil {
			results = append(results, Result{Repo: f.Source, Kind: "error", Detail: e.Error(), At: time.Now()})
			continue
		}
		norm := root
		if os.PathSeparator == '\\' {
			norm = strings.ToLower(norm)
		}
		if !seen[norm] {
			seen[norm] = true
			roots = append(roots, root)
		}
	}
	// Configuration must be usable in full before any repository is changed.
	if len(results) > 0 {
		return results
	}
	jobs := make(chan string)
	out := make(chan Result, len(roots))
	var wg sync.WaitGroup
	workers := min(len(roots), 4)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for root := range jobs {
				out <- syncRepo(ctx, root, reset, key)
			}
		}()
	}
	go func() {
		for _, root := range roots {
			jobs <- root
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()
	byRoot := map[string]Result{}
	for r := range out {
		byRoot[r.Repo] = r
	}
	for _, root := range roots {
		results = append(results, byRoot[root])
	}
	return results
}
func syncRepo(ctx context.Context, root string, reset bool, key string) (r Result) {
	r = Result{Repo: root, Kind: "error", At: time.Now()}
	fail := func(e error) Result { r.Detail = e.Error(); return r }
	gitDir, e := git(ctx, root, "", "rev-parse", "--absolute-git-dir")
	if e != nil {
		return fail(e)
	}
	gitDir = filepath.FromSlash(gitDir)
	lock := flock.New(filepath.Join(gitDir, "dfman.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return fail(e)
	}
	if !ok {
		r.Kind = "busy"
		r.Detail = "Another dfman process is syncing this repository."
		return r
	}
	defer lock.Close()
	for _, marker := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer"} {
		if _, e = os.Stat(filepath.Join(gitDir, marker)); e == nil {
			r.Kind = "conflict"
			r.Detail = "Finish or abort the existing Git operation: " + marker
			return r
		}
	}
	paths, e := git(ctx, root, "", "diff", "--name-only", "--diff-filter=U")
	if e != nil {
		return fail(e)
	}
	if paths != "" {
		r.Kind = "conflict"
		r.Detail = "Resolve existing conflicts:\n" + paths
		return r
	}
	if info, e := os.Stat(filepath.Join(root, ".gitmodules")); e == nil && info.Size() > 0 {
		return fail(fmt.Errorf("repositories with submodules require manual synchronization"))
	}
	head, e := git(ctx, root, "", "rev-parse", "--verify", "HEAD")
	if e != nil {
		return fail(e)
	}
	branch := ""
	if reset {
		refs, e := git(ctx, root, key, "ls-remote", "--symref", "origin", "HEAD")
		if e != nil {
			return fail(e)
		}
		for _, line := range strings.Split(refs, "\n") {
			f := strings.Fields(line)
			if len(f) == 3 && f[0] == "ref:" && f[2] == "HEAD" {
				branch = strings.TrimPrefix(f[1], "refs/heads/")
				break
			}
		}
		if branch == "" {
			return fail(fmt.Errorf("origin has no advertised default branch"))
		}
	} else {
		branch, e = git(ctx, root, "", "symbolic-ref", "--quiet", "--short", "HEAD")
		if e != nil {
			return fail(fmt.Errorf("select a local branch before syncing: %w", e))
		}
	}
	if _, e = git(ctx, root, key, "fetch", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch); e != nil {
		return fail(e)
	}
	remote, e := git(ctx, root, "", "rev-parse", "refs/remotes/origin/"+branch)
	if e != nil {
		return fail(e)
	}
	if reset {
		changes, e := git(ctx, root, "", "status", "--porcelain", "--untracked-files=all")
		if e != nil {
			return fail(e)
		}
		current, _ := git(ctx, root, "", "symbolic-ref", "--quiet", "--short", "HEAD")
		if head == remote && changes == "" && current == branch {
			r.Kind = "ok"
			r.Detail = "Already matches origin/" + branch
			return r
		}
		oldBranch, _ := git(ctx, root, "", "rev-parse", "--verify", "refs/heads/"+branch)
		a, e := ancestor(ctx, root, head, remote)
		if e != nil {
			return fail(e)
		}
		r.LocalSaved = changes != "" || !a
		if oldBranch != "" {
			a, e = ancestor(ctx, root, oldBranch, remote)
			if e != nil {
				return fail(e)
			}
			r.LocalSaved = r.LocalSaved || !a
		}
		a, e = ancestor(ctx, root, remote, head)
		if e != nil {
			return fail(e)
		}
		pulled := !a
		var pulledFiles []FileChange
		if pulled {
			pulledFiles, e = changedFiles(ctx, root, head, remote)
			if e != nil {
				return fail(e)
			}
		}
		snapshot, ref, e := snapshotRepo(ctx, root, gitDir, head, oldBranch)
		if e != nil {
			return fail(e)
		}
		r.Backup = snapshot
		if r.LocalSaved {
			if e = writeJSON(filepath.Join(snapshot, "notice.json"), r); e != nil {
				return fail(e)
			}
		}
		for _, args := range [][]string{{"reset", "--hard"}, {"clean", "-fd"}, {"checkout", "--no-overwrite-ignore", "-B", branch, "origin/" + branch}} {
			if _, e = git(ctx, root, "", args...); e != nil {
				return fail(fmt.Errorf("reset failed; restore from %s (%s): %w", snapshot, ref, e))
			}
		}
		r.Kind = "ok"
		r.Pulled = pulled
		r.PulledFiles = pulledFiles
		r.Detail = "Reset to origin/" + branch + ". Recovery: " + snapshot
		return r
	}
	if _, e = git(ctx, root, "", "add", "-A"); e != nil {
		return fail(e)
	}
	changes, e := git(ctx, root, "", "diff", "--cached", "--name-only")
	if e != nil {
		return fail(e)
	}
	if changes != "" {
		host, _ := os.Hostname()
		if _, e = git(ctx, root, "", "commit", "-m", "sync by "+host+" at "+time.Now().Format("2006-01-02 15:04:05")); e != nil {
			return fail(e)
		}
	}
	head, e = git(ctx, root, "", "rev-parse", "HEAD")
	if e != nil {
		return fail(e)
	}
	has, e := ancestor(ctx, root, remote, head)
	if e != nil {
		return fail(e)
	}
	if !has {
		detail, e := git(ctx, root, "", "merge-tree", "--write-tree", "--name-only", head, remote)
		if e != nil {
			var ex *exec.ExitError
			if errors.As(e, &ex) && ex.ExitCode() == 1 {
				r.Kind = "conflict"
				r.Detail = "Local work is committed; remote changes conflict:\n" + detail
				return r
			}
			return fail(fmt.Errorf("cannot prepare merge (Git 2.38+ required): %w", e))
		}
		if _, e = git(ctx, root, "", "merge", "--no-edit", "--no-stat", "--no-autostash", remote); e != nil {
			if _, check := os.Stat(filepath.Join(gitDir, "MERGE_HEAD")); check == nil {
				_, abort := git(ctx, root, "", "merge", "--abort")
				if abort != nil {
					r.Kind = "conflict"
					r.Detail = "Merge failed and could not be aborted: " + abort.Error()
					return r
				}
			}
			return fail(e)
		}
		r.Pulled = true
		r.PulledFiles, e = changedFiles(ctx, root, head, "HEAD")
		if e != nil {
			return fail(e)
		}
	}
	head, e = git(ctx, root, "", "rev-parse", "HEAD")
	if e != nil {
		return fail(e)
	}
	pushedFiles, e := changedFiles(ctx, root, remote, head)
	if e != nil {
		return fail(e)
	}
	if _, e = git(ctx, root, key, "push", "origin", "HEAD:refs/heads/"+branch); e != nil {
		return fail(fmt.Errorf("push failed; local commits are preserved: %w", e))
	}
	r.Pushed = head != remote
	r.PushedFiles = pushedFiles
	r.Kind = "ok"
	r.Detail = "Synchronized with origin/" + branch
	return r
}
func snapshotRepo(ctx context.Context, root, gitDir, head, branch string) (string, string, error) {
	dir, e := os.MkdirTemp(filepath.Join(gitDir), "dfman-snapshot-")
	if e != nil {
		return "", "", e
	}
	// Move into the durable recovery directory before creating Git references.
	base := filepath.Join(gitDir, "dfman-recovery")
	if e = os.MkdirAll(base, 0700); e != nil {
		return dir, "", e
	}
	dest := filepath.Join(base, time.Now().UTC().Format("20060102T150405")+"-"+filepath.Base(dir))
	if e = os.Rename(dir, dest); e != nil {
		return dir, "", e
	}
	dir = dest
	f, e := os.OpenFile(filepath.Join(dir, "files.tar"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return dir, "", e
	}
	tw := tar.NewWriter(f)
	e = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if rel == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(p)
			if err != nil {
				return err
			}
		}
		h, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		if err = tw.WriteHeader(h); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, in)
			ce := in.Close()
			if err != nil {
				return err
			}
			return ce
		}
		return nil
	})
	closeErr := tw.Close()
	syncErr := f.Sync()
	fileErr := f.Close()
	if e != nil {
		return dir, "", e
	}
	if closeErr != nil {
		return dir, "", closeErr
	}
	if syncErr != nil {
		return dir, "", syncErr
	}
	if fileErr != nil {
		return dir, "", fileErr
	}
	if _, e = os.Stat(filepath.Join(gitDir, "index")); e == nil {
		if e = copyPath(filepath.Join(gitDir, "index"), filepath.Join(dir, "index")); e != nil {
			return dir, "", e
		}
	}
	ref := "refs/dfman/recovery/" + filepath.Base(dir)
	if _, e = git(ctx, root, "", "update-ref", ref+"/head", head); e != nil {
		return dir, ref, e
	}
	if branch != "" {
		if _, e = git(ctx, root, "", "update-ref", ref+"/branch", branch); e != nil {
			return dir, ref, e
		}
	}
	e = os.WriteFile(filepath.Join(dir, "README"), []byte("Repository: "+root+"\nHEAD: "+head+"\nRecovery ref: "+ref+"/head\n"), 0600)
	return dir, ref, e
}
