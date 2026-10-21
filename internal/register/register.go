// Package register installs and verifies the AgentDock registration pointer in
// a project's coding-agent instruction files (AGENTS.md / CLAUDE.md).
//
// Registration is the only link between a coding agent and this engine: the
// project's root instruction file points the agent at agent/AGENT.md, which
// carries the identity, the one rule, and the skill index. Everything the agent
// does after that is driven from there. Writing that pointer by hand is a
// manual step that silently rots; this package makes it an idempotent,
// verifiable operation instead.
package register

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	// BeginMarker opens the managed block.
	BeginMarker = "<!-- agentdock:register:begin -->"
	// EndMarker closes the managed block.
	EndMarker = "<!-- agentdock:register:end -->"
	// DefaultFile is created when the project has no instruction file yet.
	DefaultFile = "AGENTS.md"
)

// Block is the managed section written into a project's instruction file.
// The agent contract lives in agent/AGENT.md; this block only names it.
const Block = BeginMarker + "\n" +
	"## AgentDock — agent-driven worktree & environment infrastructure\n" +
	"\n" +
	"Before any worktree, port, environment, or Docker work, read\n" +
	"`~/.agentdock/agent/AGENT.md` (or the vendored `agent/AGENT.md` in this project)\n" +
	"and follow its skills: bootstrap · develop · diagnose · finish · reclaim.\n" +
	"\n" +
	"The one rule: **Never guess. Verify, or refuse.**\n" +
	"\n" +
	"<!-- Managed by `agentdock register` — edits inside this block are overwritten. -->\n" +
	EndMarker

// FileStatus reports the registration state of one instruction file.
type FileStatus struct {
	File       string
	Existed    bool
	Registered bool
}

// existingTargets returns the instruction files AgentDock knows how to
// register into, in priority order. Only files that already exist are
// returned; an empty result means the project has no instruction file yet.
func existingTargets(dir string) []string {
	var out []string
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			out = append(out, name)
		}
	}
	return out
}

// Apply upserts the managed block into every existing instruction file. If none
// exists it creates AGENTS.md, the cross-runtime default. Apply is idempotent:
// running it twice produces byte-identical files.
func Apply(dir string) ([]FileStatus, error) {
	targets := existingTargets(dir)
	if len(targets) == 0 {
		targets = []string{DefaultFile}
	}
	var out []FileStatus
	for _, name := range targets {
		path := filepath.Join(dir, name)
		old, err := os.ReadFile(path)
		existed := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(upsert(string(old))), 0o644); err != nil {
			return nil, err
		}
		out = append(out, FileStatus{File: name, Existed: existed, Registered: true})
	}
	return out, nil
}

// Check reports whether the instruction files carry a complete managed block.
// It never writes. When the project has no instruction file at all, it reports
// a missing AGENTS.md so callers can fail closed.
func Check(dir string) ([]FileStatus, error) {
	targets := existingTargets(dir)
	if len(targets) == 0 {
		return []FileStatus{{File: DefaultFile, Existed: false, Registered: false}}, nil
	}
	var out []FileStatus
	for _, name := range targets {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		out = append(out, FileStatus{File: name, Existed: true, Registered: hasBlock(string(data))})
	}
	return out, nil
}

// Remove deletes the managed block from every instruction file that has one.
// Content outside the markers is preserved verbatim.
func Remove(dir string) ([]FileStatus, error) {
	var out []FileStatus
	for _, name := range existingTargets(dir) {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		updated := removeBlock(string(data))
		if updated != string(data) {
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return nil, err
			}
		}
		out = append(out, FileStatus{File: name, Existed: true, Registered: hasBlock(updated)})
	}
	return out, nil
}

// upsert replaces an existing managed block in place, or prepends one.
func upsert(content string) string {
	if i := strings.Index(content, BeginMarker); i >= 0 {
		if j := strings.Index(content[i:], EndMarker); j >= 0 {
			end := i + j + len(EndMarker)
			return content[:i] + Block + content[end:]
		}
		return content[:i] + Block
	}
	if strings.TrimSpace(content) == "" {
		return Block + "\n"
	}
	return Block + "\n\n" + content
}

// hasBlock reports whether content carries both markers, begin before end.
func hasBlock(content string) bool {
	i := strings.Index(content, BeginMarker)
	if i < 0 {
		return false
	}
	return strings.Index(content[i:], EndMarker) >= 0
}

// removeBlock deletes the managed block, keeping everything else.
func removeBlock(content string) string {
	i := strings.Index(content, BeginMarker)
	if i < 0 {
		return content
	}
	j := strings.Index(content[i:], EndMarker)
	var rest string
	if j < 0 {
		rest = content[:i]
	} else {
		rest = content[:i] + content[i+j+len(EndMarker):]
	}
	return strings.TrimLeft(rest, "\n")
}
