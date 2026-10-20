// Package config defines the .agentdock/config.json schema.
//
// Default() is EMPTY on purpose: the service list is never hardcoded. It always
// comes from discovery (`agentdock discover`) via `init --from`. This is the fix
// for eff-agent-si's original bug, where config.Default() silently wrote four
// guessed services (api/web/db/redis) regardless of the actual stack.
//
// PortBase/PortStride are configuration defaults, NOT guesses about the
// project: they only shape how host ports are allocated for isolated docks.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const DirName = ".agentdock"
const FileName = "config.json"

// DefaultPortBase is where the first dock's host ports start.
const DefaultPortBase = 41000

// DefaultPortStride is the gap between consecutive docks' port blocks.
const DefaultPortStride = 200

// Service is one discovered compose service.
type Service struct {
	Name          string   `json:"name"`
	Image         string   `json:"image,omitempty"`
	ContainerName string   `json:"container_name,omitempty"`
	Ports         []string `json:"ports"`
	Profiles      []string `json:"profiles"`
	EnvFiles      []string `json:"env_files"`
	RequiredEnv   []string `json:"required_env"`
}

// Auth describes a project's admin-login mechanism. It is recorded by the
// agent (after reading the project), never guessed by the engine. When Detected
// is true, the dock engine refuses to create a workstation without explicit
// credentials (--admin-password / --use-default-auth).
type Auth struct {
	Detected    bool   `json:"detected"`
	PasswordEnv string `json:"password_env,omitempty"`
	UsernameEnv string `json:"username_env,omitempty"`
	SeedHint    string `json:"seed_hint,omitempty"`
}

// Config is the whole .agentdock/config.json.
type Config struct {
	Version    int       `json:"version"`
	Project    string    `json:"project"`
	PortBase   int       `json:"port_base"`
	PortStride int       `json:"port_stride"`
	Services   []Service `json:"services"`
	Auth       *Auth     `json:"auth,omitempty"`
}

// Default returns an empty config. No services, no guesses.
func Default() Config {
	return Config{
		Version:    1,
		PortBase:   DefaultPortBase,
		PortStride: DefaultPortStride,
		Services:   []Service{},
	}
}

// Save writes the config under dir/.agentdock/config.json.
func (c *Config) Save(dir string) error {
	d := filepath.Join(dir, DirName)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, FileName), data, 0o644)
}

// Load reads dir/.agentdock/config.json.
func Load(dir string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(filepath.Join(dir, DirName, FileName))
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.normalize()
	return cfg, nil
}

// normalize fills zero-value fields with defaults (covers configs written by
// older versions or with missing fields).
func (c *Config) normalize() {
	if c.PortBase <= 0 {
		c.PortBase = DefaultPortBase
	}
	if c.PortStride <= 0 {
		c.PortStride = DefaultPortStride
	}
	if c.Services == nil {
		c.Services = []Service{}
	}
}

// EnsureGitignore appends the runtime state directory (".agentdock/") to
// dir/.gitignore so it never dirties the repository's git status (which would
// otherwise trip fail-closed checks like merge's dirty guard). Idempotent: a
// second call is a no-op; a missing .gitignore is created.
func EnsureGitignore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	entry := DirName + "/"
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == DirName || t == entry {
			return nil // already ignored
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(entry + "\n")
	return err
}
