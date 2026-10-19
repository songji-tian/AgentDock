// Package flow implements the finish-line commands: submit (commit + push a
// dock's worktree) and merge (fold a dock's branch back into the main line).
//
// Everything fails closed: a missing worktree, a dirty main checkout, a
// missing remote, or a merge conflict is a refusal with a specific message —
// the agent completes the request, never bypasses the engine.
package flow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/songji-tian/agentdock/internal/config"
)

// SubmitOpts controls `submit`.
type SubmitOpts struct {
	// Target is "local" (commit only) or "github" (commit + push).
	Target string
	// Remote is an optional explicit remote URL/name for the push.
	Remote string
	// Visibility is "private" (default) or "public"; used only when a remote
	// repo must be created.
	Visibility string
	// Message is the commit message; defaults to "agentdock submit <slug>".
	Message string
}

// SubmitResult reports what submit did.
type SubmitResult struct {
	Branch    string
	Commit    string
	Pushed    bool
	RemoteURL string
	RepoURL   string
	Created   bool
}

// MergeOpts controls `merge`.
type MergeOpts struct {
	// Mode is "direct" (local merge) or "pr" (open a pull request).
	Mode string
	// Remote is an optional explicit remote URL/name for the PR.
	Remote string
	// Base is the target branch; defaults to "main".
	Base string
}

// MergeResult reports what merge did.
type MergeResult struct {
	Mode   string
	Merged bool
	Commit string
	PRURL  string
}

// Submit commits the dock's worktree changes, then pushes when target is
// "github". A missing remote repo is created via `gh repo create` using
// opts.Visibility (default "private").
func Submit(dir, slug string, opts SubmitOpts) (SubmitResult, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return SubmitResult{}, err
	}
	st, err := config.LoadState(abs)
	if err != nil {
		return SubmitResult{}, err
	}
	d := st.Find(slug)
	if d == nil {
		return SubmitResult{}, fmt.Errorf("no dock %q — run `agentdock new %s` first", slug, slug)
	}
	if _, err := os.Stat(d.Worktree); err != nil {
		return SubmitResult{}, fmt.Errorf("worktree missing at %s: %w", d.Worktree, err)
	}

	msg := opts.Message
	if msg == "" {
		msg = "agentdock submit " + slug
	}
	commit, err := commitWorktree(d.Worktree, msg)
	if err != nil {
		return SubmitResult{}, err
	}
	res := SubmitResult{Branch: d.Branch, Commit: commit}

	if opts.Target != "github" {
		return res, nil
	}

	remote, err := resolveRemote(abs, opts.Remote)
	if err != nil {
		return res, err
	}
	exists, err := remoteExists(remote)
	if err != nil {
		return res, err
	}
	if !exists {
		vis := opts.Visibility
		if vis == "" {
			vis = "private"
		}
		if vis != "private" && vis != "public" {
			return res, fmt.Errorf("invalid --visibility %q (want private|public)", vis)
		}
		repoURL, err := createRepo(remote, vis)
		if err != nil {
			return res, err
		}
		res.Created = true
		res.RepoURL = repoURL
	}
	if err := push(d.Worktree, d.Branch, remote); err != nil {
		return res, err
	}
	res.Pushed = true
	res.RemoteURL = remote
	return res, nil
}

// Merge folds the dock's branch back into the main line: "direct" does a local
// merge (refuses on a dirty checkout, wrong branch, or conflict); "pr" opens a
// pull request via `gh pr create` (the branch must already be pushed).
func Merge(dir, slug string, opts MergeOpts) (MergeResult, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return MergeResult{}, err
	}
	st, err := config.LoadState(abs)
	if err != nil {
		return MergeResult{}, err
	}
	d := st.Find(slug)
	if d == nil {
		return MergeResult{}, fmt.Errorf("no dock %q — run `agentdock new %s` first", slug, slug)
	}
	base := opts.Base
	if base == "" {
		base = "main"
	}

	switch opts.Mode {
	case "direct":
		commit, err := mergeDirect(abs, d.Branch, base)
		if err != nil {
			return MergeResult{}, err
		}
		return MergeResult{Mode: "direct", Merged: true, Commit: commit}, nil
	case "pr":
		remote, err := resolveRemote(abs, opts.Remote)
		if err != nil {
			return MergeResult{}, err
		}
		prURL, err := openPR(abs, d.Branch, base, remote)
		if err != nil {
			return MergeResult{}, err
		}
		return MergeResult{Mode: "pr", PRURL: prURL}, nil
	default:
		return MergeResult{}, fmt.Errorf("invalid --mode %q (want direct|pr)", opts.Mode)
	}
}

func commitWorktree(worktree, msg string) (string, error) {
	out, err := runGit(worktree, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("nothing to commit in %s", worktree)
	}
	if _, err := runGit(worktree, "add", "-A"); err != nil {
		return "", err
	}
	if _, err := runGit(worktree, "commit", "-m", msg); err != nil {
		return "", err
	}
	sha, err := runGit(worktree, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func mergeDirect(dir, branch, base string) (string, error) {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) != "" {
		return "", fmt.Errorf("main checkout %s is dirty; commit or stash first", dir)
	}
	cur, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cur) != base {
		return "", fmt.Errorf("main checkout is on %q, not %q; switch to %q first", strings.TrimSpace(cur), base, base)
	}
	if _, err := runGit(dir, "merge", "--no-ff", "-m", "agentdock merge "+branch, branch); err != nil {
		return "", fmt.Errorf("merging %q into %q failed (conflict?): %w", branch, base, err)
	}
	sha, err := runGit(dir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func resolveRemote(dir, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	out, err := runGit(dir, "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("no remote: pass --remote URL or set `origin` on %s", dir)
	}
	return strings.TrimSpace(out), nil
}

// parseGitHubRemote splits a GitHub remote URL into owner/name.
func parseGitHubRemote(remote string) (owner, name string, ok bool) {
	s := strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	switch {
	case strings.HasPrefix(s, "git@github.com:"):
		s = strings.TrimPrefix(s, "git@github.com:")
	case strings.HasPrefix(s, "https://github.com/"):
		s = strings.TrimPrefix(s, "https://github.com/")
	case strings.HasPrefix(s, "http://github.com/"):
		s = strings.TrimPrefix(s, "http://github.com/")
	default:
		return "", "", false
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func remoteExists(remote string) (bool, error) {
	owner, name, ok := parseGitHubRemote(remote)
	if !ok {
		return false, fmt.Errorf("cannot parse GitHub remote %q", remote)
	}
	cmd := exec.Command("gh", "repo", "view", owner+"/"+name, "--json", "name")
	if _, err := cmd.Output(); err != nil {
		return false, nil
	}
	return true, nil
}

func createRepo(remote, visibility string) (string, error) {
	owner, name, ok := parseGitHubRemote(remote)
	if !ok {
		return "", fmt.Errorf("cannot parse GitHub remote %q", remote)
	}
	flag := "--private"
	if visibility == "public" {
		flag = "--public"
	}
	cmd := exec.Command("gh", "repo", "create", owner+"/"+name, flag)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("gh repo create %s/%s: %w\n%s", owner, name, err, out)
	}
	return "https://github.com/" + owner + "/" + name, nil
}

func push(worktree, branch, remote string) error {
	if _, err := runGit(worktree, "push", "-u", remote, branch); err != nil {
		return fmt.Errorf("pushing %q to %s: %w", branch, remote, err)
	}
	return nil
}

func openPR(dir, branch, base, remote string) (string, error) {
	title := fmt.Sprintf("agentdock: merge %s", branch)
	body := fmt.Sprintf("Merging dock branch %q into %q.", branch, base)
	cmd := exec.Command("gh", "pr", "create", "--repo", remote, "--base", base, "--head", branch, "--title", title, "--body", body)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w\n%s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}
