// Package cli wires the agentdock subcommands (discover, init, ...).
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

This is an agent-driven tool: register agent/AGENT.md with your coding agent and
let it drive these commands. Do not memorise flags — that is the wrong path.
`)
}
