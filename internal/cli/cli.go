// Package cli wires the agentdock subcommands (discover, init, new, up, down,
// clean, capacity, diagnose, gc, reclaim).
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/songji-tian/agentdock/internal/config"
	"github.com/songji-tian/agentdock/internal/discover"
	"github.com/songji-tian/agentdock/internal/dock"
	"github.com/songji-tian/agentdock/internal/flow"
	"github.com/songji-tian/agentdock/internal/ops"
)

// Run dispatches os.Args (after the program name).
func Run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return nil
	}
	switch args[0] {
	case "discover":
		return runDiscover(args[1:])
	case "init":
		return runInit(args[1:])
	case "new":
		return runNew(args[1:])
	case "up":
		return runUp(args[1:])
	case "down":
		return runDown(args[1:])
	case "clean":
		return runClean(args[1:])
	case "capacity":
		return runCapacity(args[1:])
	case "diagnose":
		return runDiagnose(args[1:])
	case "gc":
		return runGC(args[1:])
	case "reclaim":
		return runReclaim(args[1:])
	case "submit":
		return runSubmit(args[1:])
	case "merge":
		return runMerge(args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q (run `agentdock help`)", args[0])
	}
}

func runDiscover(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory to scan")
	asJSON := fs.Bool("json", false, "output machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	res, err := discover.Discover(*dir)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tCONTAINER_NAME\tPORTS\tREQUIRED_ENV\tPROFILES")
	for _, s := range res.Services {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			s.Name,
			dash(s.ContainerName),
			dash(strings.Join(s.Ports, ", ")),
			dash(strings.Join(s.RequiredEnv, ", ")),
			dash(strings.Join(s.Profiles, ",")))
	}
	w.Flush()
	fmt.Fprintf(os.Stderr, "\n# files: %s\n", strings.Join(res.Files, ", "))
	return nil
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	project := fs.String("project", "", "project/workstation name (defaults to dir basename)")
	from := fs.String("from", "", "discover JSON file to populate services from ('-' = stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := config.Default() // EMPTY by design — never hardcoded services
	if *project != "" {
		cfg.Project = *project
	} else {
		abs, err := filepath.Abs(*dir)
		if err != nil {
			abs = *dir
		}
		cfg.Project = filepath.Base(abs)
	}

	if *from != "" {
		var res discover.Result
		if *from == "-" {
			if err := json.NewDecoder(os.Stdin).Decode(&res); err != nil {
				return fmt.Errorf("reading discover JSON from stdin: %w", err)
			}
		} else {
			data, err := os.ReadFile(*from)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, &res); err != nil {
				return fmt.Errorf("parsing discover JSON %s: %w", *from, err)
			}
		}
		cfg.Services = res.Services
	}

	if err := cfg.Save(*dir); err != nil {
		return err
	}

	if len(cfg.Services) == 0 {
		fmt.Fprintln(os.Stderr, "⚠  config written with 0 services (empty by design — AgentDock never guesses).")
		fmt.Fprintln(os.Stderr, "   Populate it:  agentdock discover --json > d.json && agentdock init --from d.json")
		fmt.Fprintln(os.Stderr, "   Docks will refuse to start until services are discovered.")
	} else {
		fmt.Fprintf(os.Stderr, "✓ config written with %d services.\n", len(cfg.Services))
	}
	return nil
}

func runNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	adminUser := fs.String("admin-user", "", "admin username for this dock (optional; pairs with --admin-password)")
	adminPass := fs.String("admin-password", "", "admin password for this dock (required if the project has login auth)")
	useDefault := fs.Bool("use-default-auth", false, "use default admin credentials (admin / admin&798Ad)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock new [--dir DIR] [--admin-user U] [--admin-password P | --use-default-auth] SLUG")
	}
	d, err := dock.New(*dir, fs.Arg(0), dock.AuthCreds{
		UseDefault: *useDefault,
		Username:   *adminUser,
		Password:   *adminPass,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ dock %q created\n", d.Slug)
	fmt.Fprintf(os.Stderr, "  branch:   %s\n", d.Branch)
	fmt.Fprintf(os.Stderr, "  worktree: %s\n", d.Worktree)
	fmt.Fprintf(os.Stderr, "  portBase: %d\n", d.PortBase)
	return nil
}

func runUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock up [--dir DIR] SLUG")
	}
	if err := dock.Up(*dir, fs.Arg(0)); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ dock %q is up\n", fs.Arg(0))
	return nil
}

func runDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock down [--dir DIR] SLUG")
	}
	if err := dock.Down(*dir, fs.Arg(0)); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ dock %q is down\n", fs.Arg(0))
	return nil
}

func runClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock clean [--dir DIR] SLUG")
	}
	if err := dock.Clean(*dir, fs.Arg(0)); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ dock %q removed\n", fs.Arg(0))
	return nil
}

func runCapacity(args []string) error {
	fs := flag.NewFlagSet("capacity", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := ops.CapacityReport(*dir)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "PORT BASE\t%d\n", c.PortBase)
	fmt.Fprintf(w, "PORT STRIDE\t%d\n", c.PortStride)
	fmt.Fprintf(w, "DOCKS\t%d\n", len(c.Docks))
	fmt.Fprintf(w, "NEXT BASE\t%d\n", c.NextBase)
	fmt.Fprintf(w, "BLOCKS LEFT\t%d\n", c.BlocksLeft)
	w.Flush()
	if len(c.Docks) > 0 {
		fmt.Fprintln(os.Stdout)
		w2 := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w2, "SLUG\tSTATUS\tPORT_BASE\tPUBLISHED_PORTS")
		for _, r := range c.Docks {
			fmt.Fprintf(w2, "%s\t%s\t%d\t%d\n", r.Slug, r.Status, r.PortBase, r.Ports)
		}
		w2.Flush()
	}
	return nil
}

func runDiagnose(args []string) error {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: agentdock diagnose [--dir DIR] [SLUG]")
	}
	slug := ""
	if fs.NArg() == 1 {
		slug = fs.Arg(0)
	}
	findings, err := ops.Diagnose(*dir, slug)
	if err != nil {
		return err
	}
	for _, f := range findings {
		fmt.Printf("%-5s %-12s %s\n", f.Status, f.Slug, f.Detail)
	}
	return nil
}

func runGC(args []string) error {
	fs := flag.NewFlagSet("gc", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	yes := fs.Bool("yes", false, "actually reclaim the garbage (default: list only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	items, err := ops.GC(*dir, *yes)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, "✓ no garbage found")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tKIND\tDETAIL")
	for _, it := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", it.Slug, it.Kind, it.Detail)
	}
	w.Flush()
	if !*yes {
		fmt.Fprintln(os.Stderr, "\n# dry run — pass --yes to actually reclaim these")
	}
	return nil
}

func runReclaim(args []string) error {
	fs := flag.NewFlagSet("reclaim", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock reclaim [--dir DIR] SLUG")
	}
	if err := ops.Reclaim(*dir, fs.Arg(0)); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "✓ dock %q reclaimed\n", fs.Arg(0))
	return nil
}

func runSubmit(args []string) error {
	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	target := fs.String("target", "local", "local | github")
	remote := fs.String("remote", "", "remote URL/name for the push (github target)")
	visibility := fs.String("visibility", "private", "private | public (only when creating a repo)")
	msg := fs.String("m", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock submit [--target local|github] [--remote URL] [--visibility private|public] [-m MSG] SLUG")
	}
	res, err := flow.Submit(*dir, fs.Arg(0), flow.SubmitOpts{
		Target: *target, Remote: *remote, Visibility: *visibility, Message: *msg,
	})
	if err != nil {
		return err
	}
	if res.Pushed {
		what := "pushed"
		if res.Created {
			what = "repo created + pushed"
		}
		fmt.Fprintf(os.Stderr, "✓ dock %q submitted: %s %s → %s (%s)\n", fs.Arg(0), res.Commit, res.Branch, res.RemoteURL, what)
	} else {
		fmt.Fprintf(os.Stderr, "✓ dock %q committed: %s on %s\n", fs.Arg(0), res.Commit, res.Branch)
	}
	return nil
}

func runMerge(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	dir := fs.String("dir", ".", "repository directory")
	mode := fs.String("mode", "direct", "direct | pr")
	remote := fs.String("remote", "", "remote URL/name for the PR")
	base := fs.String("base", "main", "target branch")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: agentdock merge [--mode direct|pr] [--remote URL] [--base MAIN] SLUG")
	}
	res, err := flow.Merge(*dir, fs.Arg(0), flow.MergeOpts{Mode: *mode, Remote: *remote, Base: *base})
	if err != nil {
		return err
	}
	if res.Mode == "pr" {
		fmt.Fprintf(os.Stderr, "✓ PR opened: %s\n", res.PRURL)
	} else {
		fmt.Fprintf(os.Stderr, "✓ merged into %s at %s\n", *base, res.Commit)
	}
	return nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `agentdock — isolated, reproducible, parallel dev environments for coding agents.

Usage:
  agentdock discover [--dir DIR] [--json]   read the compose stack (facts, no guessing)
  agentdock init     [--dir DIR] [--project NAME] [--from FILE]
                                             write .agentdock/config.json (empty by default)
  agentdock new      [--dir DIR] SLUG        create a worktree + port block + override
  agentdock up       [--dir DIR] SLUG        validate then start the dock (fail-closed)
  agentdock down     [--dir DIR] SLUG        stop the dock
  agentdock clean    [--dir DIR] SLUG        remove the dock (refuses if still up)
  agentdock capacity [--dir DIR]             report the port ledger + free space
  agentdock diagnose [--dir DIR] [SLUG]      check dock consistency (read-only)
  agentdock gc       [--dir DIR] [--yes]     find (and with --yes reclaim) garbage docks
  agentdock reclaim  [--dir DIR] SLUG        stop + clean a finished dock in one shot
  agentdock submit   [--target local|github] [--remote URL] [--visibility private|public] [-m MSG] SLUG
  agentdock merge    [--mode direct|pr] [--remote URL] [--base MAIN] SLUG

This is an agent-driven tool: register agent/AGENT.md with your coding agent and
let it drive these commands. Do not memorise flags — that is the wrong path.
`)
}
