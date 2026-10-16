package dock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songji-tian/agentdock/internal/config"
)

// TestWriteOverrideUsesOverrideTag guards the critical isolation fix: without
// `!override`, compose merge APPENDS the ports sequence (and ports' uniqueness
// key includes the host `published` port), so the remapped port would be added
// next to the original instead of replacing it — the dock would still grab the
// original host port. Regression test for compose-spec 13-merge.md "Replace
// value".
func TestWriteOverrideUsesOverrideTag(t *testing.T) {
	dir := t.TempDir()
	services := []config.Service{
		{Name: "api", Ports: []string{"3000:80"}},
		{Name: "worker"},
	}
	alloc := map[string][]string{
		"api":    {"41000:80"},
		"worker": {},
	}
	if err := writeOverride(dir, "MyProject", "demo", services, alloc); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, overrideFileName))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "ports: !override") {
		t.Fatalf("override must use !override tag to replace base ports, got:\n%s", s)
	}
	if !strings.Contains(s, `"41000:80"`) {
		t.Fatalf("override missing remapped port, got:\n%s", s)
	}
	// A service with no published ports must NOT get a ports key at all.
	if strings.Contains(s, "worker:") && strings.Contains(s[strings.Index(s, "worker:"):], "ports:") {
		t.Fatalf("worker should have no ports key, got:\n%s", s)
	}
	// Container name must be prefixed and lower-cased.
	if !strings.Contains(s, `"myproject-demo-api"`) {
		t.Fatalf("container_name should be lowercased and prefixed, got:\n%s", s)
	}
}

// TestSupplyEnvRelativeDirSymlinkResolves guards a real-world bug: with a
// relative --dir (the CLI default is "."), supplyEnv used to create a symlink
// whose target was the relative ".env" — which resolves against the worktree
// directory itself, producing a broken, self-referential link. The target must
// be absolute so it resolves from anywhere.
func TestSupplyEnvRelativeDirSymlinkResolves(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wt := filepath.Join(repo, config.DirName, "worktrees", "rel")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("POSTGRES_PASSWORD=test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	// Relative dir, exactly like the CLI's default --dir .
	if err := supplyEnv(".", filepath.Join(config.DirName, "worktrees", "rel")); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(config.DirName, "worktrees", "rel", ".env")
	tgt, err := os.Readlink(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(tgt) {
		t.Fatalf("symlink target must be absolute (relative targets break when resolved from the worktree dir), got %q", tgt)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("symlink should resolve to the repo .env, got: %v", err)
	}
}
