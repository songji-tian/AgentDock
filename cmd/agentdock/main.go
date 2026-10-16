// Package main is the AgentDock engine entry point.
//
// AgentDock is agent-driven: a coding agent reads agent/AGENT.md and the
// skills, then drives this binary. The core loop (discover / init / new / up /
// down / clean) is implemented across internal/ packages.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "agentdock: core loop under construction (see docs/SPEC.md)")
	os.Exit(1)
}
