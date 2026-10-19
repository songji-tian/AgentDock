package ops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/songji-tian/agentdock/internal/config"
	"github.com/songji-tian/agentdock/internal/dock"
)

func writeFixture(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	cfg := config.Config{Version: 1, Project: "proj", PortBase: 41000, PortStride: 200, Services: []config.Service{}}
	if err := cfg.Save(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func saveState(t *testing.T, dir string, docks []config.Dock) {
	t.Helper()
	st := config.State{Version: 1, Docks: docks}
	if err := st.Save(dir); err != nil {
		t.Fatal(err)
	}
}

func wtPath(dir, slug string) string {
	return filepath.Join(dir, config.DirName, "worktrees", slug)
}

func TestCapacityReport(t *testing.T) {
	dir := writeFixture(t)
	saveState(t, dir, []config.Dock{
		{Slug: "a", Status: "up", PortBase: 41000, Ports: map[string][]string{"api": {"41000:80", "41001:81"}}},
		{Slug: "b", Status: "down", PortBase: 41200, Ports: map[string][]string{"web": {"41200:80"}}},
	})
	c, err := CapacityReport(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.NextBase != 41400 {
		t.Fatalf("NextBase = %d, want 41400", c.NextBase)
	}
	if len(c.Docks) != 2 {
		t.Fatalf("len(Docks) = %d, want 2", len(c.Docks))
	}
	// Sorted by port base: a then b.
	if c.Docks[0].Ports != 2 || c.Docks[1].Ports != 1 {
		t.Fatalf("published-port counts wrong: %+v", c.Docks)
	}
	wantBlocks := (65535 - 41400) / 200
	if c.BlocksLeft != wantBlocks {
		t.Fatalf("BlocksLeft = %d, want %d", c.BlocksLeft, wantBlocks)
	}
}

func TestDiagnoseMissingWorktree(t *testing.T) {
	dir := writeFixture(t)
	saveState(t, dir, []config.Dock{{Slug: "ghost", Status: "down", Worktree: wtPath(dir, "ghost")}})
	fs, err := Diagnose(dir, "ghost")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 || fs[0].Status != "err" {
		t.Fatalf("want a single err finding, got %+v", fs)
	}
}

func TestDiagnosePresentMissingEnv(t *testing.T) {
	dir := writeFixture(t)
	wt := wtPath(dir, "d1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, dock.OverrideFileName), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	saveState(t, dir, []config.Dock{{Slug: "d1", Status: "down", Worktree: wt}})
	fs, err := Diagnose(dir, "d1")
	if err != nil {
		t.Fatal(err)
	}
	var errs, warns int
	for _, f := range fs {
		switch f.Status {
		case "err":
			errs++
		case "warn":
			warns++
		}
	}
	if errs != 0 {
		t.Fatalf("expected no err findings, got %+v", fs)
	}
	if warns != 1 {
		t.Fatalf("expected one warn (.env missing), got %+v", fs)
	}
}

func TestGCList(t *testing.T) {
	dir := writeFixture(t)
	saveState(t, dir, []config.Dock{
		{Slug: "ghost", Status: "down", Worktree: wtPath(dir, "ghost")},
		{Slug: "abandoned", Status: "down", Worktree: wtPath(dir, "abandoned")},
		{Slug: "live", Status: "up", Worktree: wtPath(dir, "live")},
	})
	// abandoned and live have real dirs; stray is a dir with no ledger entry.
	for _, s := range []string{"abandoned", "live", "stray"} {
		if err := os.MkdirAll(wtPath(dir, s), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	items, err := GC(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, it := range items {
		kinds[it.Slug] = it.Kind
	}
	if kinds["ghost"] != "orphan-ledger" {
		t.Fatalf("ghost kind = %q, want orphan-ledger", kinds["ghost"])
	}
	if kinds["abandoned"] != "abandoned-down" {
		t.Fatalf("abandoned kind = %q, want abandoned-down", kinds["abandoned"])
	}
	if kinds["stray"] != "orphan-worktree" {
		t.Fatalf("stray kind = %q, want orphan-worktree", kinds["stray"])
	}
	if _, ok := kinds["live"]; ok {
		t.Fatalf("live up dock must be left alone (unverified), got %q", kinds["live"])
	}
}

func TestGCOrphanLedgerExecute(t *testing.T) {
	dir := writeFixture(t)
	saveState(t, dir, []config.Dock{{Slug: "ghost", Status: "down", Worktree: wtPath(dir, "ghost")}})
	items, err := GC(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "orphan-ledger" {
		t.Fatalf("items = %+v", items)
	}
	st, err := config.LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Find("ghost") != nil {
		t.Fatalf("ghost should be removed from the ledger")
	}
}

func TestReclaimNoSuchDock(t *testing.T) {
	dir := writeFixture(t)
	if err := Reclaim(dir, "nope"); err == nil {
		t.Fatal("want error for unknown slug")
	}
}
