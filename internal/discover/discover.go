// Package discover deterministically parses a repository's docker-compose
// files and reports facts: services, ports, container names, required env vars.
//
// It exists because LLMs mis-read YAML (they dropped 443 and 3001 on a real
// stack). A deterministic parser cannot make that mistake. The agent must use
// this output as the single source of truth instead of eyeballing the file.
package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/songji-tian/agentdock/internal/config"
	"gopkg.in/yaml.v3"
)

var defaultBaseNames = []string{
	"compose.yaml", "compose.yml",
	"docker-compose.yaml", "docker-compose.yml",
}

var overrideNames = []string{
	"compose.override.yaml", "compose.override.yml",
	"docker-compose.override.yaml", "docker-compose.override.yml",
}

type Result struct {
	Files    []string         `json:"files"`
	Services []config.Service `json:"services"`
}

// Discover parses the default compose file set in dir (one base file plus one
// override file, matching what `docker compose up` auto-loads) and returns the
// merged service list.
func Discover(dir string) (*Result, error) {
	files := findDefaultFiles(dir)
	if len(files) == 0 {
		return nil, fmt.Errorf("no docker-compose file found in %s", dir)
	}

	merged := map[string]*yaml.Node{}
	for _, f := range files {
		svcs, err := parseServices(f)
		if err != nil {
			return nil, err
		}
		for name, node := range svcs {
			merged[name] = mergeService(merged[name], node)
		}
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	res := &Result{Files: files}
	for _, name := range names {
		res.Services = append(res.Services, extractService(name, merged[name]))
	}
	return res, nil
}

func findDefaultFiles(dir string) []string {
	var files []string
	for _, n := range defaultBaseNames {
		p := filepath.Join(dir, n)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			files = append(files, p)
			break
		}
	}
	for _, n := range overrideNames {
		p := filepath.Join(dir, n)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			files = append(files, p)
			break
		}
	}
	return files
}

// DefaultFiles returns the compose file set that `docker compose up` auto-loads
// (one base file + one override file, in merge order). The dock engine passes
// these (plus its own override) to `docker compose` explicitly.
func DefaultFiles(dir string) []string {
	return findDefaultFiles(dir)
}

// parseServices reads one compose file and returns service name -> raw node.
func parseServices(path string) (map[string]*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top level is not a mapping", path)
	}
	svcNode := mapValue(root, "services")
	if svcNode == nil || svcNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: no `services` mapping", path)
	}
	out := map[string]*yaml.Node{}
	for i := 0; i+1 < len(svcNode.Content); i += 2 {
		out[svcNode.Content[i].Value] = svcNode.Content[i+1]
	}
	return out, nil
}

func extractService(name string, node *yaml.Node) config.Service {
	s := config.Service{
		Name:        name,
		Ports:       []string{},
		Profiles:    []string{},
		EnvFiles:    []string{},
		RequiredEnv: []string{},
	}
	if node == nil || node.Kind != yaml.MappingNode {
		return s
	}
	if v := mapValue(node, "image"); v != nil {
		s.Image = scalar(v)
	}
	if v := mapValue(node, "container_name"); v != nil {
		s.ContainerName = scalar(v)
	}
	if v := mapValue(node, "ports"); v != nil {
		s.Ports = portStrings(v)
	}
	if v := mapValue(node, "profiles"); v != nil {
		s.Profiles = seqScalars(v)
	}
	if v := mapValue(node, "env_file"); v != nil {
		s.EnvFiles = seqScalars(v)
	}
	s.RequiredEnv = requiredEnv(node)
	return s
}

// mergeService returns base with override keys winning. It is a shallow
// key-level merge (good enough for ports/container_name/env facts).
func mergeService(base, over *yaml.Node) *yaml.Node {
	if base == nil {
		return over
	}
	if over == nil {
		return base
	}
	merged := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(base.Content); i += 2 {
		if mapValue(over, base.Content[i].Value) == nil {
			merged.Content = append(merged.Content, base.Content[i], base.Content[i+1])
		}
	}
	merged.Content = append(merged.Content, over.Content...)
	return merged
}

// portStrings extracts "host:container" strings from a ports node, handling the
// scalar form ("5432:5432") and the mapping form ({published:5432, target:5432}).
func portStrings(n *yaml.Node) []string {
	out := []string{}
	switch n.Kind {
	case yaml.SequenceNode:
		for _, item := range n.Content {
			switch item.Kind {
			case yaml.ScalarNode:
				if s := strings.TrimSpace(item.Value); s != "" {
					out = append(out, s)
				}
			case yaml.MappingNode:
				p, t := mapValue(item, "published"), mapValue(item, "target")
				if p != nil && t != nil {
					out = append(out, scalar(p)+":"+scalar(t))
				} else if t != nil {
					out = append(out, scalar(t))
				}
			}
		}
	case yaml.ScalarNode:
		if s := strings.TrimSpace(n.Value); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func seqScalars(n *yaml.Node) []string {
	out := []string{}
	switch n.Kind {
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if item.Kind == yaml.ScalarNode {
				out = append(out, item.Value)
			}
		}
	case yaml.ScalarNode:
		out = append(out, n.Value)
	}
	return out
}

// envRefRe matches compose variable interpolation: ${NAME}, ${NAME:-def},
// ${NAME-def}, ${NAME:?err}, ${NAME?err}, ${NAME:+alt}, ${NAME+alt}.
var envRefRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:?[-?+][^}]*)?\}`)

// requiredEnv walks every scalar in the service and reports variables referenced
// with the required syntax (${VAR:?...} / ${VAR?...}).
func requiredEnv(node *yaml.Node) []string {
	seen := map[string]bool{}
	out := []string{}
	walkScalars(node, func(s string) {
		for _, m := range envRefRe.FindAllStringSubmatch(s, -1) {
			name, mod := m[1], m[2]
			if strings.HasPrefix(mod, "?") || (len(mod) >= 2 && mod[1] == '?') {
				if !seen[name] {
					seen[name] = true
					out = append(out, name)
				}
			}
		}
	})
	sort.Strings(out)
	return out
}

func walkScalars(n *yaml.Node, fn func(string)) {
	if n == nil {
		return
	}
	if n.Kind == yaml.ScalarNode {
		fn(n.Value)
	}
	for _, c := range n.Content {
		walkScalars(c, fn)
	}
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func scalar(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}
