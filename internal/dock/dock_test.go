package dock

import (
	"fmt"
	"net"
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
	b, err := os.ReadFile(filepath.Join(dir, OverrideFileName))
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

// TestSupplyEnvCopiesNotSymlinks: a dock's .env is now a copy of the repo's
// (a snapshot), not a symlink, so each dock is independent of the repo.
func TestSupplyEnvCopiesNotSymlinks(t *testing.T) {
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
	if err := supplyEnv(".", filepath.Join(config.DirName, "worktrees", "rel"), nil, "", ""); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(config.DirName, "worktrees", "rel", ".env")
	fi, err := os.Lstat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf(".env must be a plain copy, not a symlink")
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "POSTGRES_PASSWORD=test\n" {
		t.Fatalf("copy should match repo .env, got %q", string(b))
	}

	// Independence: mutating the repo's .env must NOT affect the dock's copy.
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("POSTGRES_PASSWORD=rotated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(dst)
	if string(b) != "POSTGRES_PASSWORD=test\n" {
		t.Fatalf("dock .env should stay a snapshot, got %q", string(b))
	}
}

// TestSupplyEnvOverridesAuthKeys: when auth is detected, the admin
// username/password keys are overridden with the values the user chose.
func TestSupplyEnvOverridesAuthKeys(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wt := filepath.Join(repo, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte(
		"POSTGRES_PASSWORD=dbpass\nADMIN_PASSWORD=oldpass\nADMIN_USERNAME=oldadmin\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	auth := &config.Auth{Detected: true, PasswordEnv: "ADMIN_PASSWORD", UsernameEnv: "ADMIN_USERNAME"}
	if err := supplyEnv(repo, wt, auth, "admin", "admin&798Ad"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(wt, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "ADMIN_PASSWORD=admin&798Ad") {
		t.Fatalf("password must be overridden (literal &), got:\n%s", s)
	}
	if !strings.Contains(s, "ADMIN_USERNAME=admin") {
		t.Fatalf("username must be overridden, got:\n%s", s)
	}
	if !strings.Contains(s, "POSTGRES_PASSWORD=dbpass") {
		t.Fatalf("unrelated keys must be preserved, got:\n%s", s)
	}
	if strings.Contains(s, "oldpass") || strings.Contains(s, "oldadmin") {
		t.Fatalf("old credentials must be gone, got:\n%s", s)
	}
}

// TestSupplyEnvOverridesExistingTrackedEnv: when the repo tracks .env, a fresh
// worktree checks it out, so dst already exists. supplyEnv must still override
// the admin keys (regression: the old early-return left the repo's credentials
// in the dock).
func TestSupplyEnvOverridesExistingTrackedEnv(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wt := filepath.Join(repo, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("ADMIN_PASSWORD=repo_secret\nADMIN_USERNAME=repo_admin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// dst already present (as if tracked and checked out into the worktree).
	if err := os.WriteFile(filepath.Join(wt, ".env"), []byte("ADMIN_PASSWORD=repo_secret\nADMIN_USERNAME=repo_admin\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	auth := &config.Auth{Detected: true, PasswordEnv: "ADMIN_PASSWORD", UsernameEnv: "ADMIN_USERNAME"}
	if err := supplyEnv(repo, wt, auth, "admin", "admin&798Ad"); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(wt, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "repo_secret") || strings.Contains(s, "repo_admin") {
		t.Fatalf("repo credentials must not leak into the dock, got:\n%s", s)
	}
	if !strings.Contains(s, "ADMIN_PASSWORD=admin&798Ad") || !strings.Contains(s, "ADMIN_USERNAME=admin") {
		t.Fatalf("dock .env must carry the chosen credentials, got:\n%s", s)
	}
}

func TestSetEnvKey(t *testing.T) {
	cases := []struct {
		name    string
		content string
		key     string
		value   string
		want    string
	}{
		{"append to empty", "", "ADMIN_PASSWORD", "x", "ADMIN_PASSWORD=x\n"},
		{"append when missing", "A=1\n", "ADMIN_PASSWORD", "x", "A=1\nADMIN_PASSWORD=x\n"},
		{"replace existing", "A=1\nADMIN_PASSWORD=old\n", "ADMIN_PASSWORD", "x", "A=1\nADMIN_PASSWORD=x\n"},
		{"replace export form", "export ADMIN_PASSWORD=old\n", "ADMIN_PASSWORD", "x", "ADMIN_PASSWORD=x\n"},
		{"literal ampersand", "A=1\n", "ADMIN_PASSWORD", "admin&798Ad", "A=1\nADMIN_PASSWORD=admin&798Ad\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := string(setEnvKey([]byte(c.content), c.key, c.value))
			if got != c.want {
				t.Fatalf("setEnvKey(%q) = %q, want %q", c.content, got, c.want)
			}
		})
	}
}

func TestResolveAuth(t *testing.T) {
	noAuth := config.Config{}
	auth := config.Config{Auth: &config.Auth{Detected: true, PasswordEnv: "ADMIN_PASSWORD", UsernameEnv: "ADMIN_USERNAME"}}

	if _, _, err := resolveAuth(noAuth, AuthCreds{}); err != nil {
		t.Fatalf("no auth + no creds should not error, got %v", err)
	}
	if _, _, err := resolveAuth(auth, AuthCreds{}); err == nil {
		t.Fatalf("auth detected + no creds must refuse (fail-closed)")
	}
	u, p, err := resolveAuth(auth, AuthCreds{UseDefault: true})
	if err != nil || u != "admin" || p != "admin&798Ad" {
		t.Fatalf("default creds = %q/%q, err=%v", u, p, err)
	}
	u, p, err = resolveAuth(auth, AuthCreds{Password: "hunter2"})
	if err != nil || u != "" || p != "hunter2" {
		t.Fatalf("password-only = %q/%q, err=%v", u, p, err)
	}
	u, p, err = resolveAuth(auth, AuthCreds{Username: "root", Password: "hunter2"})
	if err != nil || u != "root" || p != "hunter2" {
		t.Fatalf("user+pass = %q/%q, err=%v", u, p, err)
	}
}

// TestNextPortBaseSkipsOccupiedBlock guards the cross-project port collision
// fix: when the base block's ports are already bound on the host (by another
// project's dock or an unrelated process), nextPortBase must advance a full
// stride to the next block instead of reusing the occupied ports.
func TestNextPortBaseSkipsOccupiedBlock(t *testing.T) {
	const base = 60000
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", base))
	if err != nil {
		t.Skipf("cannot occupy port %d for test: %v", base, err)
	}
	defer l.Close()

	cfg := config.Config{
		PortBase:   base,
		PortStride: 200,
		Services: []config.Service{
			{Name: "api", Ports: []string{"8000:8000"}},
		},
	}
	st := config.State{}
	got := nextPortBase(cfg, st)
	want := base + 200
	if got != want {
		t.Errorf("nextPortBase = %d, want %d (must skip occupied block)", got, want)
	}
}
