package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const StateFileName = "state.json"

// State is the .agentdock/state.json ledger of docks (workstations).
type State struct {
	Version int    `json:"version"`
	Docks   []Dock `json:"docks"`
}

// Dock is one isolated workstation.
type Dock struct {
	Slug     string              `json:"slug"`
	Branch   string              `json:"branch"`
	Worktree string              `json:"worktree"`
	PortBase int                 `json:"port_base"`
	Ports    map[string][]string `json:"ports"`  // service -> ["HOST:CONTAINER", ...]
	Status   string              `json:"status"` // "up" | "down"
}

// LoadState reads dir/.agentdock/state.json (empty state if absent).
func LoadState(dir string) (State, error) {
	st := State{Version: 1, Docks: []Dock{}}
	data, err := os.ReadFile(filepath.Join(dir, DirName, StateFileName))
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	if st.Docks == nil {
		st.Docks = []Dock{}
	}
	return st, nil
}

// Save writes dir/.agentdock/state.json.
func (st *State) Save(dir string) error {
	d := filepath.Join(dir, DirName)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, StateFileName), data, 0o644)
}

// Find returns the dock with the given slug, or nil.
func (st *State) Find(slug string) *Dock {
	for i := range st.Docks {
		if st.Docks[i].Slug == slug {
			return &st.Docks[i]
		}
	}
	return nil
}

// Upsert inserts or replaces a dock by slug.
func (st *State) Upsert(d Dock) {
	for i := range st.Docks {
		if st.Docks[i].Slug == d.Slug {
			st.Docks[i] = d
			return
		}
	}
	st.Docks = append(st.Docks, d)
}

// Remove deletes a dock by slug and reports whether it existed.
func (st *State) Remove(slug string) bool {
	for i := range st.Docks {
		if st.Docks[i].Slug == slug {
			st.Docks = append(st.Docks[:i], st.Docks[i+1:]...)
			return true
		}
	}
	return false
}
