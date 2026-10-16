# ADR 0001: The product is an agent; the engine is its hands

## Status
Accepted.

## Context
A pure CLI tool with a good README is exactly the failure mode we are
replacing: humans (and coding agents) memorise commands incompletely, skip
steps, and produce wrong results silently. The previous tool (eff-agent-si)
shipped both a "register the agent" path and a "just run the CLI" path side by
side, and `init` wrote a hardcoded service list — so users took the CLI path
and got a wrong config without realising it.

## Decision
AgentDock is two halves:

- `agent/` — instructions a coding agent reads (identity, rules, skills).
- `agentdock` — a deterministic engine that executes and refuses.

The human registers the agent; the agent drives the engine. The engine's
commands are listed in the README only for transparency, not as a manual path.

## Consequences
- The "agent-driven" path is the only documented path.
- The engine must be safe to drive: empty `init`, deterministic `discover`,
  fail-closed `new`/`up`.
