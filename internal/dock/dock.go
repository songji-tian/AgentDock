// Package dock creates, starts, stops and removes isolated workstations.
//
// It is the "engine" half of AgentDock: it executes deterministic operations
// (worktree, ports, compose override) and refuses — fail-closed — whenever it
// cannot prove the operation is safe. The agent proposes; the engine verifies.
package dock

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/songji-tian/agentdock/internal/config"
	"github.com/songji-tian/agentdock/internal/discover"
	"github.com/songji-tian/agentdock/internal/ports"
)

const OverrideFileName = "docker-compose.agentdock.yml"

// New creates a workstation: validates config, allocates a port block, creates
// a git worktree, writes the compose override and supplies .env.
func New(dir, slug string, creds AuthCreds) (*config.Dock, error) {
	if !validSlug(slug) {
		return nil, fmt.Errorf("invalid slug %q (use lowercase letters, digits and hyphens)", slug)
	}
	abs, err := absDir(dir)
	if err != nil {
		return nil, err
	}
	dir = abs
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load config (run `agentdock init` first): %w", err)
	}
	if len(cfg.Services) == 0 {
		return nil, fmt.Errorf("no services in config — run `agentdock discover` + `agentdock init --from` first")
	}

	// Auth fail-closed: the engine never invents admin credentials. When the
	// project has login, New must have been told the credentials explicitly.
	username, password, err := resolveAuth(cfg, creds)
	if err != nil {
		return nil, err
	}
	warnAuthHints(cfg, dir)

	st, err := config.LoadState(dir)
	if err != nil {
		return nil, err
	}
	if st.Find(slug) != nil {
		return nil, fmt.Errorf("dock %q already exists (use `agentdock up %s` or pick a new slug)", slug, slug)
	}

	portBase := nextPortBase(cfg, st)
	alloc := ports.Allocate(cfg.Services, portBase)

	branch := "agentdock/" + slug
	worktree := filepath.Join(dir, config.DirName, "worktrees", slug)

	if err := createWorktree(dir, branch, worktree); err != nil {
		return nil, err
	}
	if err := writeOverride(worktree, cfg.Project, slug, cfg.Services, alloc); err != nil {
		return nil, err
	}
	if err := supplyEnv(dir, worktree, cfg.Auth, username, password); err != nil {
		return nil, err
	}

	d := config.Dock{
		Slug:     slug,
		Branch:   branch,
		Worktree: worktree,
		PortBase: portBase,
		Ports:    alloc,
		Status:   "down",
	}
	st.Upsert(d)
	if err := st.Save(dir); err != nil {
		return nil, err
	}
	return &d, nil
}

// Up validates the dock's config then starts it (fail-closed).
func Up(dir, slug string) error {
	abs, err := absDir(dir)
	if err != nil {
		return err
	}
	dir = abs
	d, err := loadDock(dir, slug)
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	// Re-supply .env only if the dock's is missing (the repo's may have appeared
	// after `new`). Never clobber an existing .env: it holds the credentials the
	// user chose at `new` time.
	envPath := filepath.Join(d.Worktree, ".env")
	if _, err := os.Lstat(envPath); os.IsNotExist(err) {
		if err := supplyEnv(dir, d.Worktree, cfg.Auth, "", ""); err != nil {
			return err
		}
	}
	files, err := composeFiles(d.Worktree)
	if err != nil {
		return err
	}
	proj := projectName(cfg.Project, slug)

	// Validate first: refuse before starting anything if the config is bad
	// (missing required env, unresolved placeholders, broken override).
	v := dockerCompose(proj, files, "config", "-q")
	v.Dir = d.Worktree
	if out, err := v.CombinedOutput(); err != nil {
		return fmt.Errorf("config invalid (refusing to start): %w\n%s", err, out)
	}

	up := dockerCompose(proj, files, "up", "-d")
	up.Dir = d.Worktree
	if out, err := up.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose up: %w\n%s", err, out)
	}

	st, err := config.LoadState(dir)
	if err != nil {
		return err
	}
	d.Status = "up"
	st.Upsert(*d)
	return st.Save(dir)
}

// Down stops the dock's containers.
func Down(dir, slug string) error {
	abs, err := absDir(dir)
	if err != nil {
		return err
	}
	dir = abs
	d, err := loadDock(dir, slug)
	if err != nil {
		return err
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return err
	}
	files, err := composeFiles(d.Worktree)
	if err != nil {
		return err
	}
	down := dockerCompose(projectName(cfg.Project, slug), files, "down")
	down.Dir = d.Worktree
	if out, err := down.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose down: %w\n%s", err, out)
	}
	st, err := config.LoadState(dir)
	if err != nil {
		return err
	}
	d.Status = "down"
	st.Upsert(*d)
	return st.Save(dir)
}

// Clean removes the dock's worktree and reclaims its ledger entry (fail-closed
// if the dock is still up).
func Clean(dir, slug string) error {
	abs, err := absDir(dir)
	if err != nil {
		return err
	}
	dir = abs
	d, err := loadDock(dir, slug)
	if err != nil {
		return err
	}
	if d.Status == "up" {
		return fmt.Errorf("dock %q is up — run `agentdock down %s` first", slug, slug)
	}
	rm := exec.Command("git", "-C", dir, "worktree", "remove", "--force", d.Worktree)
	if out, err := rm.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, out)
	}
	// Best effort: drop the branch (it is no longer checked out).
	_ = exec.Command("git", "-C", dir, "branch", "-D", d.Branch).Run()

	st, err := config.LoadState(dir)
	if err != nil {
		return err
	}
	st.Remove(slug)
	return st.Save(dir)
}

// --- internals ---

func validSlug(s string) bool {
	if s == "" || len(s) > 63 || s[0] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}
	return true
}

func loadDock(dir, slug string) (*config.Dock, error) {
	st, err := config.LoadState(dir)
	if err != nil {
		return nil, err
	}
	d := st.Find(slug)
	if d == nil {
		return nil, fmt.Errorf("no dock %q — run `agentdock new %s` first", slug, slug)
	}
	return d, nil
}

// absDir resolves dir to an absolute path so downstream relative paths
// (worktree, compose -f flags, .env symlink) stay consistent whether the user
// passed a relative --dir (the CLI default is ".") or not.
func absDir(dir string) (string, error) {
	if filepath.IsAbs(dir) {
		return dir, nil
	}
	return filepath.Abs(dir)
}

// nextPortBase returns the first host port of the next dock's block: one stride
// past the highest block already in the ledger.
func nextPortBase(cfg config.Config, st config.State) int {
	base := cfg.PortBase
	for _, d := range st.Docks {
		if d.PortBase+cfg.PortStride > base {
			base = d.PortBase + cfg.PortStride
		}
	}
	return base
}

func createWorktree(dir, branch, worktree string) error {
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "-C", dir, "worktree", "add", "-b", branch, worktree)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add: %w\n%s", err, out)
	}
	return nil
}

// writeOverride writes a compose override that remaps container names and host
// ports for this dock. The container side of every port is unchanged; only the
// host side is shifted onto the dock's block. Services with no published ports
// still get a container_name (so they can't collide with the main stack).
func writeOverride(worktree, project, slug string, services []config.Service, alloc map[string][]string) error {
	sorted := append([]config.Service(nil), services...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	prefix := strings.ToLower(project) + "-" + slug
	var b strings.Builder
	b.WriteString("# Generated by AgentDock — do not edit.\n")
	b.WriteString("services:\n")
	for _, svc := range sorted {
		b.WriteString("  " + svc.Name + ":\n")
		b.WriteString("    container_name: " + strconv.Quote(prefix+"-"+svc.Name) + "\n")
		if ps := alloc[svc.Name]; len(ps) > 0 {
			// !override fully replaces the base ports list instead of appending.
			// Compose merge appends sequences, and ports' uniqueness key includes
			// the host `published` port — so without !override our remap would ADD
			// a second binding (and still grab the original host port), defeating
			// isolation. See compose-spec 13-merge.md "Replace value".
			b.WriteString("    ports: !override\n")
			for _, p := range ps {
				b.WriteString("      - " + strconv.Quote(p) + "\n")
			}
		}
	}
	return os.WriteFile(filepath.Join(worktree, OverrideFileName), []byte(b.String()), 0o644)
}

// AuthCreds are the admin credentials the agent passes to New after asking the
// human. The engine never invents them.
type AuthCreds struct {
	UseDefault bool
	Username   string
	Password   string
}

// Default admin credentials, used only when the user explicitly opts in with
// --use-default-auth. The '&' is a literal password character.
const (
	defaultAuthUser     = "admin"
	defaultAuthPassword = "admin&798Ad"
)

// resolveAuth turns AuthCreds into the username/password to inject, refusing
// (fail-closed) when the project has admin login but no credentials were given.
func resolveAuth(cfg config.Config, creds AuthCreds) (username, password string, err error) {
	if cfg.Auth == nil || !cfg.Auth.Detected {
		if creds.UseDefault || creds.Username != "" || creds.Password != "" {
			fmt.Fprintf(os.Stderr, "⚠  auth not detected in config; ignoring --admin-* flags\n")
		}
		return "", "", nil
	}
	switch {
	case creds.UseDefault:
		return defaultAuthUser, defaultAuthPassword, nil
	case creds.Password != "":
		return creds.Username, creds.Password, nil
	default:
		return "", "", fmt.Errorf("this project has admin login (auth.detected) and AgentDock never invents credentials. " +
			"Ask the user, then re-run with --admin-password (optionally --admin-user), or --use-default-auth (admin / admin&798Ad)")
	}
}

// warnAuthHints prints a soft warning when admin-login signals exist but the
// agent did not mark config auth. It is a backstop, not a hard block: the
// authoritative detection is the agent's own reading of the project.
func warnAuthHints(cfg config.Config, dir string) {
	if cfg.Auth != nil && cfg.Auth.Detected {
		return
	}
	sigs := discover.AuthSignals(dir)
	if len(sigs) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "⚠  admin-login signals found but config auth.detected is false: %s\n", strings.Join(sigs, ", "))
	fmt.Fprintf(os.Stderr, "   If this project has login, the agent should mark config auth before relying on isolation.\n")
}

// supplyEnv writes the dock's .env snapshot: the repo's .env with the admin
// username/password keys overridden by the credentials chosen at `new` time.
// It always writes, so a tracked .env in the repo can't leak the repo's admin
// password into the dock.
func supplyEnv(repoDir, worktree string, auth *config.Auth, username, password string) error {
	src := filepath.Join(repoDir, ".env")
	dst := filepath.Join(worktree, ".env")

	var content []byte
	if _, err := os.Stat(src); err == nil {
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return rerr
		}
		content = data
	}

	if auth != nil && auth.Detected {
		if auth.PasswordEnv != "" && password != "" {
			content = setEnvKey(content, auth.PasswordEnv, password)
		}
		if auth.UsernameEnv != "" && username != "" {
			content = setEnvKey(content, auth.UsernameEnv, username)
		}
	}

	if content == nil {
		// No repo .env and no credentials to inject: nothing to supply.
		return nil
	}
	return os.WriteFile(dst, content, 0o600)
}

// setEnvKey sets KEY=value in env-file content, replacing an existing line or
// appending a new one. Values are written verbatim (no shell interpretation).
func setEnvKey(content []byte, key, value string) []byte {
	s := strings.TrimRight(string(content), "\n")
	var lines []string
	if s != "" {
		lines = strings.Split(s, "\n")
	}
	match := key + "="
	replaced := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "export ") {
			t = strings.TrimSpace(strings.TrimPrefix(t, "export "))
		}
		if t == key || strings.HasPrefix(t, match) {
			lines[i] = key + "=" + value
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, key+"="+value)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// composeFiles returns the worktree's compose file set (base + repo override)
// plus the AgentDock override, in merge order.
func composeFiles(worktree string) ([]string, error) {
	base := discover.DefaultFiles(worktree)
	if len(base) == 0 {
		return nil, fmt.Errorf("no compose file found in %s", worktree)
	}
	files := append([]string(nil), base...)
	files = append(files, filepath.Join(worktree, OverrideFileName))
	return files, nil
}

func projectName(project, slug string) string {
	return strings.ToLower(project) + "-" + slug
}

func dockerCompose(project string, files []string, sub string, subArgs ...string) *exec.Cmd {
	args := []string{"compose", "-p", project}
	for _, f := range files {
		args = append(args, "-f", f)
	}
	args = append(args, sub)
	args = append(args, subArgs...)
	return exec.Command("docker", args...)
}

// Running reports whether the dock's compose project has any running
// containers. verified is false when the running state cannot be determined
// (docker unavailable, or the worktree has no compose files) — callers must
// treat an unverified dock as unknown, never as "not running".
func Running(dir string, d config.Dock) (running, verified bool) {
	if _, err := exec.LookPath("docker"); err != nil {
		return false, false
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return false, false
	}
	files, err := composeFiles(d.Worktree)
	if err != nil {
		return false, false
	}
	ps := dockerCompose(projectName(cfg.Project, d.Slug), files, "ps", "-q")
	ps.Dir = d.Worktree
	out, err := ps.Output()
	if err != nil {
		// Docker is reachable but the project is not running / not created.
		return false, true
	}
	return strings.TrimSpace(string(out)) != "", true
}
