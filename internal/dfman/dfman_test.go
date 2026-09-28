package dfman

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func put(t *testing.T, p, s string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
}
func get(t *testing.T, p string) string {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func g(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(t.TempDir(), "no-config"))
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v: %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}

type fixture struct{ dir, remote, local, other, target string }

func setup(t *testing.T) fixture {
	t.Helper()
	d := t.TempDir()
	f := fixture{d, filepath.Join(d, "remote.git"), filepath.Join(d, "local space"), filepath.Join(d, "other"), filepath.Join(d, "target")}
	g(t, d, "init", "--bare", "--initial-branch=main", f.remote)
	g(t, d, "clone", f.remote, f.local)
	g(t, f.local, "config", "user.name", "Test")
	g(t, f.local, "config", "user.email", "test@example.invalid")
	g(t, f.local, "config", "commit.gpgsign", "false")
	g(t, f.local, "config", "core.autocrlf", "false")
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "base\n")
	put(t, filepath.Join(f.local, ".gitignore"), "ignored\n")
	g(t, f.local, "add", ".")
	g(t, f.local, "commit", "-m", "initial")
	g(t, f.local, "push", "-u", "origin", "main")
	g(t, d, "clone", f.remote, f.other)
	g(t, f.other, "config", "user.name", "Test")
	g(t, f.other, "config", "user.email", "test@example.invalid")
	g(t, f.other, "config", "commit.gpgsign", "false")
	g(t, f.other, "config", "core.autocrlf", "false")
	return f
}
func (f fixture) folders() []Folder { return []Folder{{filepath.Join(f.local, "dotfiles"), f.target}} }
func (f fixture) push(t *testing.T, name, data string) {
	put(t, filepath.Join(f.other, name), data)
	g(t, f.other, "add", ".")
	g(t, f.other, "commit", "-m", "remote")
	g(t, f.other, "push")
}
func (f fixture) sync(reset bool) Result {
	return Sync(context.Background(), f.folders(), reset, "")[0]
}
func TestSyncNormalDivergenceAndDedup(t *testing.T) {
	f := setup(t)
	r := f.sync(false)
	if r.Code() != 0 || r.Pulled || r.Pushed {
		t.Fatalf("no-op: %+v", r)
	}
	put(t, filepath.Join(f.local, "dotfiles", ".local"), "local")
	f.push(t, "dotfiles/.remote", "remote")
	folders := append(f.folders(), Folder{f.local, f.target})
	rs := Sync(context.Background(), folders, false, "")
	if len(rs) != 1 || rs[0].Code() != 0 || !rs[0].Pulled || !rs[0].Pushed {
		t.Fatalf("diverged: %+v", rs)
	}
	if g(t, f.local, "status", "--porcelain") != "" {
		t.Fatal("dirty")
	}
	if get(t, filepath.Join(f.local, "dotfiles", ".remote")) != "remote" {
		t.Fatal("missing remote")
	}
	if f.sync(false).Pushed {
		t.Fatal("no-op pushed")
	}
}
func TestConflictsPreserveLocalCommit(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "local\n")
	f.push(t, "dotfiles/.test", "remote\n")
	r := f.sync(false)
	if r.Code() != 2 {
		t.Fatalf("%+v", r)
	}
	if get(t, filepath.Join(f.local, "dotfiles", ".test")) != "local\n" {
		t.Fatal("lost local")
	}
	if g(t, f.local, "status", "--porcelain") != "" {
		t.Fatal("not committed")
	}
	if _, e := os.Stat(filepath.Join(f.local, ".git", "MERGE_HEAD")); !os.IsNotExist(e) {
		t.Fatal("left merge")
	}
}
func TestFetchPushFailures(t *testing.T) {
	t.Run("fetch", func(t *testing.T) {
		f := setup(t)
		head := g(t, f.local, "rev-parse", "HEAD")
		put(t, filepath.Join(f.local, "dotfiles", ".test"), "local")
		g(t, f.local, "remote", "set-url", "origin", filepath.Join(f.dir, "missing"))
		r := f.sync(false)
		if r.Code() != 1 || g(t, f.local, "rev-parse", "HEAD") != head || get(t, filepath.Join(f.local, "dotfiles", ".test")) != "local" {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("push", func(t *testing.T) {
		f := setup(t)
		put(t, filepath.Join(f.local, "dotfiles", ".test"), "local")
		g(t, f.local, "remote", "set-url", "--push", "origin", filepath.Join(f.dir, "missing"))
		r := f.sync(false)
		if r.Code() != 1 || !strings.Contains(r.Detail, "local commits are preserved") || g(t, f.local, "status", "--porcelain") != "" {
			t.Fatalf("%+v", r)
		}
		if g(t, f.local, "show", "HEAD:dotfiles/.test") != "local" {
			t.Fatal("lost commit")
		}
	})
}
func TestResetRecovery(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "committed")
	g(t, f.local, "add", ".")
	g(t, f.local, "commit", "-m", "local")
	old := g(t, f.local, "rev-parse", "HEAD")
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "staged")
	g(t, f.local, "add", ".")
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "unstaged")
	put(t, filepath.Join(f.local, "untracked"), "untracked")
	put(t, filepath.Join(f.local, "ignored"), "ignored")
	f.push(t, "dotfiles/.remote", "new")
	r := f.sync(true)
	if r.Code() != 0 || !r.LocalSaved || !r.Pulled || r.Backup == "" {
		t.Fatalf("%+v", r)
	}
	if g(t, f.local, "rev-parse", "HEAD") != g(t, f.other, "rev-parse", "HEAD") {
		t.Fatal("wrong head")
	}
	refs := g(t, f.local, "for-each-ref", "--format=%(objectname)", "refs/dfman/recovery")
	if !strings.Contains(refs, old) {
		t.Fatal("lost recovery head")
	}
	if _, e := os.Stat(filepath.Join(r.Backup, "index")); e != nil {
		t.Fatal(e)
	}
	file, e := os.Open(filepath.Join(r.Backup, "files.tar"))
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	tr := tar.NewReader(file)
	saved := map[string]string{}
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(tr)
		saved[h.Name] = string(b)
	}
	for p, want := range map[string]string{"dotfiles/.test": "unstaged", "untracked": "untracked", "ignored": "ignored"} {
		if saved[p] != want {
			t.Fatalf("backup %s: %q", p, saved[p])
		}
	}
	if get(t, filepath.Join(f.local, "ignored")) != "ignored" {
		t.Fatal("ignored data removed")
	}
}
func TestOperationAndOverlap(t *testing.T) {
	f := setup(t)
	head := g(t, f.local, "rev-parse", "HEAD")
	put(t, filepath.Join(f.local, ".git", "MERGE_HEAD"), head)
	if f.sync(true).Code() != 2 {
		t.Fatal("reset touched unfinished merge")
	}
	os.Remove(filepath.Join(f.local, ".git", "MERGE_HEAD"))
	l := flock.New(filepath.Join(f.local, ".git", "dfman.lock"))
	if e := l.Lock(); e != nil {
		t.Fatal(e)
	}
	if f.sync(false).Code() != 75 {
		t.Fatal("overlap not rejected")
	}
	l.Close()
	if f.sync(false).Code() != 0 {
		t.Fatal("lock not released")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Sync(ctx, f.folders(), true, "")[0].Code() == 0 {
		t.Fatal("canceled operation succeeded")
	}
	if g(t, f.local, "rev-parse", "HEAD") != head {
		t.Fatal("interruption changed head")
	}
}
func TestInvalidConfigurationPreflight(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "changed")
	head := g(t, f.local, "rev-parse", "HEAD")
	folders := append(f.folders(), Folder{filepath.Join(f.dir, "missing"), f.target})
	rs := Sync(context.Background(), folders, false, "")
	if resultCode(rs) == 0 || g(t, f.local, "rev-parse", "HEAD") != head {
		t.Fatal("mutated valid repo despite invalid source")
	}
}
func TestLinkPriorityAndFailure(t *testing.T) {
	d := t.TempDir()
	a, b, target := filepath.Join(d, "first"), filepath.Join(d, "second"), filepath.Join(d, "target")
	put(t, filepath.Join(a, ".file"), "a")
	put(t, filepath.Join(b, ".file"), "b")
	folders := []Folder{{a, target}, {b, target}}
	opt := LinkOptions{Input: strings.NewReader(""), Output: io.Discard}
	if e := Link(folders, opt); e != nil {
		t.Fatal(e)
	}
	p, e := os.Readlink(filepath.Join(target, ".file"))
	if e != nil || !samePath(p, filepath.Join(a, ".file")) {
		t.Fatalf("priority: %s %v", p, e)
	}
	os.Remove(filepath.Join(target, ".file"))
	put(t, filepath.Join(target, ".file"), "keep")
	if e = Link(folders, opt); e != nil {
		t.Fatal(e)
	}
	if get(t, filepath.Join(target, ".file")) != "keep" {
		t.Fatal("overwritten")
	}
	put(t, filepath.Join(a, "blocked#child"), "data")
	put(t, filepath.Join(target, "blocked"), "blocking file")
	if e = Link(folders, opt); e == nil {
		t.Fatal("symlink failure hidden")
	}
	if _, e = os.Stat(filepath.Join(target, "blocked", "child")); e == nil {
		t.Fatal("copy fallback")
	}
}
func TestConfigAndCommands(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "dfman.conf")
	for _, bad := range []string{"~/dotfiles\n", "notification = 'yes'", "[agent]\ninterval='61s'", "[agent]\nsync_mode='push'", "typo=true"} {
		put(t, p, bad)
		if _, e := LoadConfig(p); e == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	c := DefaultConfig()
	c.Folders = []Folder{{d, "~"}}
	if e := SaveConfig(p, c); e != nil {
		t.Fatal(e)
	}
	got, e := LoadConfig(p)
	if e != nil || !got.Notification || got.Agent.Interval != "10m" {
		t.Fatalf("%+v %v", got, e)
	}
	for _, alias := range []string{"repo-add", "self-update", "cron", "auto-sync"} {
		if Execute([]string{alias}, "test", strings.NewReader(""), io.Discard, io.Discard) == 0 {
			t.Fatal("accepted alias", alias)
		}
	}
}
func TestAgentNotificationsAndManualSilence(t *testing.T) {
	f := setup(t)
	c := DefaultConfig()
	c.Folders = f.folders()
	state := filepath.Join(f.dir, "state")
	calls := 0
	notify := func(context.Context, string, string, string) error { calls++; return nil }
	if e := runAgent(context.Background(), c, state, io.Discard, notify); e != nil || calls != 0 {
		t.Fatalf("noop: %v %d", e, calls)
	}
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "change")
	if e := runAgent(context.Background(), c, state, io.Discard, notify); e != nil || calls != 1 {
		t.Fatalf("push: %v %d", e, calls)
	}
	g(t, f.local, "remote", "set-url", "origin", filepath.Join(f.dir, "missing"))
	for i := 0; i < 2; i++ {
		if e := runAgent(context.Background(), c, state, io.Discard, notify); e == nil {
			t.Fatal("expected error")
		}
	}
	if calls != 2 {
		t.Fatal("repeated error", calls)
	}
	g(t, f.local, "remote", "set-url", "origin", f.remote)
	if e := runAgent(context.Background(), c, state, io.Discard, notify); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "next")
	fail := func(context.Context, string, string, string) error { return errors.New("notification unavailable") }
	if e := runAgent(context.Background(), c, state, io.Discard, fail); e != nil {
		t.Fatal("notification changed Git result", e)
	}
	s, _, _ := statusText(state, false)
	if !strings.Contains(s, "notification unavailable") {
		t.Fatal(s)
	}
	config := filepath.Join(f.dir, "config")
	if e := SaveConfig(config, c); e != nil {
		t.Fatal(e)
	}
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "manual")
	for _, command := range []string{"sync", "link"} {
		var err bytes.Buffer
		if code := Execute([]string{"--config", config, "--state-dir", state, command}, "test", strings.NewReader(""), io.Discard, &err); code != 0 {
			t.Fatalf("manual %s: %d %s", command, code, err.String())
		}
	}
	s, _, _ = statusText(state, false)
	if !strings.Contains(s, "notification unavailable") {
		t.Fatal("manual command invoked notifier")
	}
}
func TestArchiveAndChecksums(t *testing.T) {
	if verifyChecksum([]byte("x"), []byte(hash("x")+"  file.zip\n"), "file.zip") != nil {
		t.Fatal("valid hash")
	}
	if verifyChecksum([]byte("tampered"), []byte(hash("x")+"  file.zip\n"), "file.zip") == nil {
		t.Fatal("bad hash")
	}
	for _, name := range []string{"../escape", "/absolute", `C:\evil`} {
		d := t.TempDir()
		p := filepath.Join(d, "a.zip")
		f, _ := os.Create(p)
		z := zip.NewWriter(f)
		w, _ := z.Create(name)
		w.Write([]byte("x"))
		z.Close()
		f.Close()
		if extractPackage(p, filepath.Join(d, "output")) == nil {
			t.Fatal("accepted traversal", name)
		}
	}
}

func TestRecoveryNoticeSurvivesInterruptedReport(t *testing.T) {
	f := setup(t)
	put(t, filepath.Join(f.local, "dotfiles", ".test"), "dirty")
	r := f.sync(true)
	if r.Code() != 0 || !r.LocalSaved {
		t.Fatalf("%+v", r)
	}
	state := filepath.Join(f.dir, "fresh-state")
	if e := recordResults(state, []Result{f.sync(true)}); e != nil {
		t.Fatal(e)
	}
	s, _, e := statusText(state, false)
	if e != nil || !strings.Contains(s, "Local differences saved") {
		t.Fatal(s, e)
	}
	statusText(state, true)
	if e = recordResults(state, []Result{f.sync(true)}); e != nil {
		t.Fatal(e)
	}
	s, _, _ = statusText(state, false)
	if strings.Contains(s, "Local differences saved") {
		t.Fatal("acknowledged notice returned")
	}
}

func TestOSLockReleasedAfterProcessExit(t *testing.T) {
	if os.Getenv("DFMAN_TEST_LOCK_CHILD") == "1" {
		l := flock.New(os.Getenv("DFMAN_TEST_LOCK_PATH"))
		if e := l.Lock(); e != nil {
			os.Exit(2)
		}
		os.Stdout.Write([]byte("locked\n"))
		time.Sleep(time.Hour)
	}
	path := filepath.Join(t.TempDir(), "lock")
	cmd := exec.Command(testExecutable(t), "-test.run=^TestOSLockReleasedAfterProcessExit$")
	cmd.Env = append(os.Environ(), "DFMAN_TEST_LOCK_CHILD=1", "DFMAN_TEST_LOCK_PATH="+path)
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer cmd.Process.Kill()
	b := make([]byte, 7)
	if _, e = io.ReadFull(pipe, b); e != nil {
		t.Fatal(e)
	}
	l := flock.New(path)
	defer l.Close()
	if ok, e := l.TryLock(); e != nil || ok {
		t.Fatal("child did not hold lock", e)
	}
	cmd.Process.Kill()
	cmd.Wait()
	if ok, e := l.TryLock(); e != nil || !ok {
		t.Fatal("lock survived process exit", e)
	}
}

func testExecutable(t *testing.T) string {
	t.Helper()
	p, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	return p
}
