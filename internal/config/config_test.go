package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignoreCreatesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), ".agentdock/") {
		t.Fatalf(".gitignore = %q, want .agentdock/", string(data))
	}
	// Second call must be a no-op (no duplicate line).
	if err := EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if got := strings.Count(string(data2), ".agentdock/"); got != 1 {
		t.Fatalf(".agentdock/ appears %d times after idempotent call, want 1", got)
	}
}

func TestEnsureGitignoreKeepsExistingEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n.env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureGitignore(dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	for _, want := range []string{"node_modules/", ".env", ".agentdock/"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf(".gitignore missing %q, got %q", want, string(data))
		}
	}
}
