// Package ops implements the v2 maintenance commands: capacity, diagnose, gc
// and reclaim.
//
// The read-only ones (capacity, diagnose) never touch anything. The
// destructive ones fail closed: `gc` only lists unless --yes is passed, and it
// never treats an unverifiable "up" dock as garbage; `reclaim` stops before
// cleaning if it cannot stop the dock first.
package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/songji-tian/agentdock/internal/config"
	"github.com/songji-tian/agentdock/internal/dock"
)

// maxHostPort is the highest host port an OS can bind (0..65535).
const maxHostPort = 65535

// --- capacity -----------------------------------------------------------

// DockRow is one dock's footprint in the port ledger.
type DockRow struct {
	Slug     string
	Status   string
	PortBase int
	Ports    int // total published host ports in this dock's block
}

// Capacity summarizes how the host port space is divided between docks.
type Capacity struct {
	PortBase   int
	PortStride int
	Docks      []DockRow
	NextBase   int
	BlocksLeft int
}

// CapacityReport reads the config + state ledger and reports the port space.
// It is read-only.
func CapacityReport(dir string) (Capacity, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Capacity{}, err
	}
	cfg, err := config.Load(abs)
	if err != nil {
		return Capacity{}, fmt.Errorf("load config (run `agentdock init` first): %w", err)
	}
	st, err := config.LoadState(abs)
	if err != nil {
		return Capacity{}, err
	}

	var rows []DockRow
	next := cfg.PortBase
	for _, d := range st.Docks {
		n := 0
		for _, ps := range d.Ports {
			n += len(ps)
		}
		rows = append(rows, DockRow{Slug: d.Slug, Status: d.Status, PortBase: d.PortBase, Ports: n})
		if d.PortBase+cfg.PortStride > next {
			next = d.PortBase + cfg.PortStride
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].PortBase < rows[j].PortBase })

	blocks := 0
	if next < maxHostPort {
		blocks = (maxHostPort - next) / cfg.PortStride
	}

	return Capacity{
		PortBase:   cfg.PortBase,
		PortStride: cfg.PortStride,
		Docks:      rows,
		NextBase:   next,
		BlocksLeft: blocks,
	}, nil
}

// --- diagnose -----------------------------------------------------------

// Finding is one evidence item from a diagnose pass.
type Finding struct {
	Slug   string
	Status string // "ok" | "warn" | "err"
	Detail string
}

// Diagnose checks a dock's (or all docks') filesystem + ledger consistency.
// It is read-only and returns evidence, never fixes anything.
func Diagnose(dir, slug string) ([]Finding, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	st, err := config.LoadState(abs)
	if err != nil {
		return nil, err
	}

	var targets []config.Dock
	if slug != "" {
		d := st.Find(slug)
		if d == nil {
			return nil, fmt.Errorf("no dock %q in ledger — run `agentdock new %s` first", slug, slug)
		}
		targets = []config.Dock{*d}
	} else {
		targets = st.Docks
	}

	var out []Finding
	for _, d := range targets {
		out = append(out, checkDock(d)...)
	}
	return out, nil
}

func checkDock(d config.Dock) []Finding {
	var fs []Finding
	add := func(status, detail string) {
		fs = append(fs, Finding{Slug: d.Slug, Status: status, Detail: detail})
	}

	if _, err := os.Stat(d.Worktree); os.IsNotExist(err) {
		add("err", "worktree missing on disk: "+d.Worktree)
		return fs
	}
	add("ok", "worktree present: "+d.Worktree)

	ov := filepath.Join(d.Worktree, dock.OverrideFileName)
	if _, err := os.Stat(ov); os.IsNotExist(err) {
		add("err", "compose override missing (the dock was never fully created)")
	} else {
		add("ok", "compose override present")
	}

	env := filepath.Join(d.Worktree, ".env")
	if _, err := os.Stat(env); os.IsNotExist(err) {
		add("warn", ".env missing (will be re-supplied on `up`)")
	} else {
		add("ok", ".env present")
	}

	add("ok", "ledger status="+d.Status)
	return fs
}

// --- gc -----------------------------------------------------------------

// Garbage is one reclaimable artifact found by gc.
type Garbage struct {
	Slug   string
	Kind   string // "orphan-ledger" | "abandoned-down" | "stale-up" | "orphan-worktree"
	Detail string
}

// GC scans the ledger + filesystem for garbage. With execute=false (the
// default) it only lists. With execute=true it reclaims each item, stopping at
// the first failure so a bad item never cascades.
func GC(dir string, execute bool) ([]Garbage, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	st, err := config.LoadState(abs)
	if err != nil {
		return nil, err
	}

	var items []Garbage
	wtRoot := filepath.Join(abs, config.DirName, "worktrees")

	// 1. Ledger entries whose backing files are gone or stale.
	for _, d := range st.Docks {
		_, statErr := os.Stat(d.Worktree)
		switch {
		case os.IsNotExist(statErr):
			items = append(items, Garbage{Slug: d.Slug, Kind: "orphan-ledger", Detail: "ledger entry without a worktree"})
		case d.Status == "down":
			items = append(items, Garbage{Slug: d.Slug, Kind: "abandoned-down", Detail: "down but worktree was never cleaned"})
		case d.Status == "up":
			running, verified := dock.Running(abs, d)
			if verified && !running {
				items = append(items, Garbage{Slug: d.Slug, Kind: "stale-up", Detail: "ledger says up but no containers running"})
			}
			// Unverified "up" docks are left alone: we cannot prove they are stale.
		}
	}

	// 2. Worktree directories on disk with no ledger entry.
	if entries, err := os.ReadDir(wtRoot); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if st.Find(e.Name()) != nil {
				continue
			}
			items = append(items, Garbage{Slug: e.Name(), Kind: "orphan-worktree", Detail: "worktree directory without a ledger entry"})
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })

	if !execute {
		return items, nil
	}
	for _, it := range items {
		if err := reclaimOne(abs, &st, it); err != nil {
			return items, fmt.Errorf("reclaiming %q (%s): %w", it.Slug, it.Kind, err)
		}
	}
	return items, nil
}

func reclaimOne(abs string, st *config.State, it Garbage) error {
	switch it.Kind {
	case "orphan-ledger":
		st.Remove(it.Slug)
		return st.Save(abs)
	case "abandoned-down":
		return dock.Clean(abs, it.Slug)
	case "stale-up":
		if err := dock.Down(abs, it.Slug); err != nil {
			return err
		}
		return dock.Clean(abs, it.Slug)
	case "orphan-worktree":
		wt := filepath.Join(abs, config.DirName, "worktrees", it.Slug)
		rm := exec.Command("git", "-C", abs, "worktree", "remove", "--force", wt)
		if out, err := rm.CombinedOutput(); err != nil {
			return fmt.Errorf("git worktree remove: %w\n%s", err, out)
		}
		_ = exec.Command("git", "-C", abs, "branch", "-D", "agentdock/"+it.Slug).Run()
		return nil
	default:
		return fmt.Errorf("unknown garbage kind %q", it.Kind)
	}
}

// --- reclaim ------------------------------------------------------------

// Reclaim tears a finished dock down in one shot: stop it (if still up), then
// clean it. It stops before cleaning if the dock cannot be stopped.
func Reclaim(dir, slug string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	st, err := config.LoadState(abs)
	if err != nil {
		return err
	}
	d := st.Find(slug)
	if d == nil {
		return fmt.Errorf("no dock %q — run `agentdock new %s` first", slug, slug)
	}
	if d.Status == "up" {
		if err := dock.Down(abs, slug); err != nil {
			return fmt.Errorf("stopping dock: %w", err)
		}
	}
	return dock.Clean(abs, slug)
}
