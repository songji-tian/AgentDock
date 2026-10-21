package register

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplyCreatesDefaultFile(t *testing.T) {
	dir := t.TempDir()
	statuses, err := Apply(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].File != DefaultFile || !statuses[0].Registered {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
	if got := read(t, dir, DefaultFile); !hasBlock(got) {
		t.Fatalf("no managed block: %q", got)
	}
}

func TestApplyPrependsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "AGENTS.md", "# Project\n\nrules\n")
	if _, err := Apply(dir); err != nil {
		t.Fatal(err)
	}
	got := read(t, dir, "AGENTS.md")
	if !strings.HasPrefix(got, BeginMarker) {
		t.Fatalf("block not at top: %q", got)
	}
	if !strings.Contains(got, "# Project") {
		t.Fatal("original content lost")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "AGENTS.md", "# P\n")
	if _, err := Apply(dir); err != nil {
		t.Fatal(err)
	}
	first := read(t, dir, "AGENTS.md")
	if _, err := Apply(dir); err != nil {
		t.Fatal(err)
	}
	if second := read(t, dir, "AGENTS.md"); second != first {
		t.Fatalf("not idempotent:\nfirst=%q\nsecond=%q", first, second)
	}
}

func TestApplyUpdatesStaleBlock(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "AGENTS.md", BeginMarker+"\nold\n"+EndMarker+"\n\ntail\n")
	if _, err := Apply(dir); err != nil {
		t.Fatal(err)
	}
	got := read(t, dir, "AGENTS.md")
	if strings.Contains(got, "old") {
		t.Fatal("stale block not replaced")
	}
	if !strings.Contains(got, "tail") {
		t.Fatal("content after block lost")
	}
	if strings.Count(got, BeginMarker) != 1 {
		t.Fatal("duplicate managed block")
	}
}

func TestApplyWritesBothExistingFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "AGENTS.md", "# A\n")
	write(t, dir, "CLAUDE.md", "# C\n")
	statuses, err := Apply(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 {
		t.Fatalf("want 2 files, got %d", len(statuses))
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if !hasBlock(read(t, dir, name)) {
			t.Fatalf("%s not registered", name)
		}
	}
}

func TestCheckFailsClosed(t *testing.T) {
	dir := t.TempDir()
	statuses, err := Check(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].Registered {
		t.Fatalf("expected missing AGENTS.md, got %+v", statuses)
	}
	write(t, dir, "AGENTS.md", "# P\n")
	if statuses, _ = Check(dir); statuses[0].Registered {
		t.Fatal("unregistered file reported as registered")
	}
	if _, err := Apply(dir); err != nil {
		t.Fatal(err)
	}
	if statuses, _ = Check(dir); !statuses[0].Registered {
		t.Fatal("registered file reported missing")
	}
}

func TestRemoveRestoresOriginal(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "AGENTS.md", "# P\n\nrules\n")
	if _, err := Apply(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "AGENTS.md"); got != "# P\n\nrules\n" {
		t.Fatalf("remove did not restore original: %q", got)
	}
}

func TestApplyRefusesIncompleteBlock(t *testing.T) {
	dir := t.TempDir()
	original := "# Project\n\nkeep me\n" + BeginMarker + "\nunclosed tail\n"
	write(t, dir, "AGENTS.md", original)
	if _, err := Apply(dir); !errors.Is(err, ErrIncompleteBlock) {
		t.Fatalf("want ErrIncompleteBlock, got %v", err)
	}
	if got := read(t, dir, "AGENTS.md"); got != original {
		t.Fatalf("file changed on refusal:\n got %q\nwant %q", got, original)
	}
}

func TestRemoveRefusesIncompleteBlock(t *testing.T) {
	dir := t.TempDir()
	original := "# Project\n\nkeep me\n" + BeginMarker + "\nunclosed tail\n"
	write(t, dir, "AGENTS.md", original)
	if _, err := Remove(dir); !errors.Is(err, ErrIncompleteBlock) {
		t.Fatalf("want ErrIncompleteBlock, got %v", err)
	}
	if got := read(t, dir, "AGENTS.md"); got != original {
		t.Fatalf("file changed on refusal:\n got %q\nwant %q", got, original)
	}
}

func TestApplyIncompleteBlockLeavesOtherFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	good := "# A\n"
	write(t, dir, "AGENTS.md", good)
	write(t, dir, "CLAUDE.md", BeginMarker+"\nunclosed tail\n")
	if _, err := Apply(dir); !errors.Is(err, ErrIncompleteBlock) {
		t.Fatalf("want ErrIncompleteBlock, got %v", err)
	}
	if got := read(t, dir, "AGENTS.md"); got != good {
		t.Fatalf("sibling file written before refusal: %q", got)
	}
}
