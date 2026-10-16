// Package config defines the .agentdock/config.json schema.
//
// Default() is EMPTY on purpose: the service list is never hardcoded. It always
// comes from discovery (`agentdock discover`) via `init --from`. This is the fix
// for eff-agent-si's original bug, where config.Default() silently wrote four
// guessed services (api/web/db/redis) regardless of the actual stack.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DirName = ".agentdock"
const FileName = "config.json"

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

// Config is the whole .agentdock/config.json.
type Config struct {
	Version  int       `json:"version"`
	Project  string    `json:"project"`
	Services []Service `json:"services"`
}

// Default returns an empty config. No services, no guesses.
func Default() Config {
	return Config{Version: 1, Services: []Service{}}
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
	return cfg, nil
}
