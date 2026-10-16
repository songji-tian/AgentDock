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

const overrideFileName = "docker-compose.agentdock.yml"

// New creates a workstation: validates config, allocates a port block, creates
// a git worktree, writes the compose override and supplies .env.
func New(dir, slug string) (*config.Dock, error) {
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
	if err := supplyEnv(dir, worktree); err != nil {
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
	// Re-supply .env (idempotent): the repo's .env may have appeared or been
	// rotated after `new`; the worktree needs it for required vars.
	if err := supplyEnv(dir, d.Worktree); err != nil {
		return err
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
	return os.WriteFile(filepath.Join(worktree, overrideFileName), []byte(b.String()), 0o644)
}

// supplyEnv makes the repo's .env visible inside the worktree (it is normally
// gitignored, so a fresh worktree lacks it and required vars like
// POSTGRES_PASSWORD would be missing). Symlink so both always agree.
func supplyEnv(repoDir, worktree string) error {
	src := filepath.Join(repoDir, ".env")
	if _, err := os.Stat(src); err != nil {
		return nil // no .env in the repo; compose defaults must suffice
	}
	// The symlink target must be absolute: a relative target is resolved
	// against the symlink's own directory (the worktree), so a relative
	// `--dir .` would produce a self-referential, broken link.
	if !filepath.IsAbs(src) {
		abs, err := filepath.Abs(src)
		if err != nil {
			return err
		}
		src = abs
	}
	dst := filepath.Join(worktree, ".env")
	if _, err := os.Lstat(dst); err == nil {
		return nil // already present (tracked or supplied earlier)
	}
	if err := os.Symlink(src, dst); err != nil {
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(dst, data, 0o600)
	}
	return nil
}

// composeFiles returns the worktree's compose file set (base + repo override)
// plus the AgentDock override, in merge order.
func composeFiles(worktree string) ([]string, error) {
	base := discover.DefaultFiles(worktree)
	if len(base) == 0 {
		return nil, fmt.Errorf("no compose file found in %s", worktree)
	}
	files := append([]string(nil), base...)
	files = append(files, filepath.Join(worktree, overrideFileName))
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
