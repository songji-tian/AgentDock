package flow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songji-tian/agentdock/internal/config"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// newRepo creates a fresh git repo on branch main with one commit.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".agentdock/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "init")
	return dir
}

// newDock creates a worktree on branch agentdock/<slug> and records it in the
// ledger, returning the worktree path.
func newDock(t *testing.T, dir, slug string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt-"+slug)
	branch := "agentdock/" + slug
	git(t, dir, "worktree", "add", "-b", branch, wt)
	st := config.State{Version: 1, Docks: []config.Dock{
		{Slug: slug, Branch: branch, Worktree: wt, Status: "down"},
	}}
	if err := st.Save(dir); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestSubmitLocal(t *testing.T) {
	dir := newRepo(t)
	wt := newDock(t, dir, "demo")
	if err := os.WriteFile(filepath.Join(wt, "app.txt"), []byte("change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Submit(dir, "demo", SubmitOpts{Target: "local", Message: "my commit"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit == "" {
		t.Fatal("empty commit hash")
	}
	if res.Pushed {
		t.Fatal("local target must not push")
	}
	log := git(t, wt, "log", "--oneline", "-1")
	if !strings.Contains(log, "my commit") {
		t.Fatalf("commit log = %q", log)
	}
}

func TestSubmitNothingToCommit(t *testing.T) {
	dir := newRepo(t)
	newDock(t, dir, "demo")
	if _, err := Submit(dir, "demo", SubmitOpts{Target: "local"}); err == nil {
		t.Fatal("want error for nothing to commit")
	}
}

func TestSubmitUnknownSlug(t *testing.T) {
	dir := newRepo(t)
	if _, err := Submit(dir, "nope", SubmitOpts{Target: "local"}); err == nil {
		t.Fatal("want error for unknown slug")
	}
}

func TestMergeDirect(t *testing.T) {
	dir := newRepo(t)
	wt := newDock(t, dir, "demo")
	if err := os.WriteFile(filepath.Join(wt, "app.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-qm", "feature work")

	res, err := Merge(dir, "demo", MergeOpts{Mode: "direct", Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Merged || res.Commit == "" {
		t.Fatalf("res = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.txt")); err != nil {
		t.Fatal("app.txt was not merged into main")
	}
}

func TestMergeDirectDirty(t *testing.T) {
	dir := newRepo(t)
	newDock(t, dir, "demo")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Merge(dir, "demo", MergeOpts{Mode: "direct", Base: "main"}); err == nil {
		t.Fatal("want error for dirty main checkout")
	}
}

func TestMergeDirectWrongBranch(t *testing.T) {
	dir := newRepo(t)
	newDock(t, dir, "demo")
	git(t, dir, "checkout", "-q", "-b", "other")
	if _, err := Merge(dir, "demo", MergeOpts{Mode: "direct", Base: "main"}); err == nil {
		t.Fatal("want error when not on base branch")
	}
}

func TestParseGitHubRemote(t *testing.T) {
	cases := []struct {
		in, owner, name string
		ok              bool
	}{
		{"git@github.com:o/r.git", "o", "r", true},
		{"https://github.com/o/r", "o", "r", true},
		{"https://github.com/o/r.git", "o", "r", true},
		{"http://github.com/o/r.git", "o", "r", true},
		{"git@gitlab.com:o/r.git", "", "", false},
		{"not-a-remote", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		owner, name, ok := parseGitHubRemote(c.in)
		if owner != c.owner || name != c.name || ok != c.ok {
			t.Fatalf("parseGitHubRemote(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, owner, name, ok, c.owner, c.name, c.ok)
		}
	}
}
