# AgentDock

**Isolated, reproducible, parallel development environments for AI coding agents — driven by the agent, not memorised by the human.**

AgentDock lets multiple AI coding agents work on the same repository in parallel:
each task gets its own git worktree, its own port range, and its own Docker stack.
No collisions, reproducible, reclaimable.

> **AgentDock is an agent, not a CLI you memorise.** You do not learn a list of
> commands. You register AgentDock into your coding agent, and your coding agent
> drives it. If you are about to type `agentdock ...` yourself, you are holding
> it wrong — read [Getting started](#getting-started).

---

## The problem

Most "parallel AI development" setups break the moment two agents run at once:
they share one checkout, one database, one set of ports. The second agent
overwrites the first's work, or both stacks fight over port 80 and 443.

## What AgentDock does

Each unit of work gets a **dock** — a self-contained environment:

- its own **git worktree** (isolated checkout on its own branch)
- its own **port range** (deterministically assigned, never 80/443)
- its own **Docker stack** (own containers, own names, own volumes)

When the work is done, the dock is torn down and everything is reclaimed.

## How it works — two halves, one product

```
your coding agent (Codex / Claude Code / Cursor / OpenCode / …)
        │  reads
        ▼
  agent/AGENT.md        ← who it is, what it must never do
  agent/skills/         ← how to bootstrap / develop / diagnose / reclaim / finish
        │  drives
        ▼
  agentdock (the engine)  ← deterministic execution: discover · new · up · down · clean · submit · merge · capacity · diagnose · gc · reclaim
```

The agent brings **judgement**. The engine brings **guarantees**. Keep them
separate. The engine never guesses; when the agent's request is incomplete or
unsafe, the engine **refuses** instead of building a broken stack.

## Getting started

There is exactly one supported path. Do not skip it.

### 1. Install the engine

```sh
go install github.com/songji-tian/agentdock/cmd/agentdock@latest
agentdock version
```

### 2. Register the agent with your coding agent

Keep a clone of this repository somewhere stable, e.g. `~/.agentdock/`, then
let the engine write the pointer for you:

```sh
agentdock register --dir /path/to/your/project
agentdock register --dir /path/to/your/project --check   # verify (CI-friendly)
```

`register` writes a managed block into the project's `AGENTS.md` (and
`CLAUDE.md` when it exists) pointing at `~/.agentdock/agent/AGENT.md`. The block
is idempotent and removable (`--remove`); one command instead of hand-editing.

Which file your runtime reads:

| Runtime | Instruction file |
|---|---|
| Codex (and increasingly others) | `AGENTS.md` |
| Claude Code | `CLAUDE.md` / `.claude/` |
| Cursor | `.cursor/rules/*.mdc` |

See [agent/REGISTRATION.md](agent/REGISTRATION.md) for the manual fallback and
the full procedure.

### 3. Verify it registered

Ask your agent: *"what is the one rule?"* A registered AgentDock answers
**"Never guess. Verify, or refuse."** If it does not, registration failed — fix
it before doing anything else.

### 4. Bootstrap your repository

In your project, tell your coding agent: *"set up AgentDock in this repo."*
It runs the **bootstrap** skill: `agentdock discover` reads your compose and
lists every service/port/container-name/required-variable it found, then shows
you the list **once** for confirmation, then writes it into `.agentdock/config.json`.

`agentdock init` writes nothing on its own — it is empty by design. The service
list always comes from discovery + your confirmation, never from a hardcoded
default.

## The engine, for reference

The commands below are **what your agent runs on your behalf**. You should not
need to memorise them; they are listed here for transparency and debugging.

| Command | What it does |
|---|---|
| `agentdock discover` | Parse the repo's compose and print the facts (services, ports, container names, required env) |
| `agentdock init` | Create `.agentdock/` — empty; accepts `--from <facts>` |
| `agentdock register` | Write (or verify with `--check`, remove with `--remove`) the agent pointer in `AGENTS.md` / `CLAUDE.md` |
| `agentdock new <slug>` | Create a worktree + assign a port range + write the override |
| `agentdock up` | Start the dock's stack (verifies first; refuses if incomplete) |
| `agentdock down` | Stop the dock's stack |
| `agentdock clean` | Tear down the worktree and reclaim ports |
| `agentdock capacity` | Show the port ledger and how many dock slots remain |
| `agentdock diagnose <slug>` | Check one dock's consistency (worktree / override / .env / ledger) |
| `agentdock gc [--yes]` | Find (and with --yes reclaim) garbage docks |
| `agentdock reclaim <slug>` | Stop + clean a finished dock in one shot |
| `agentdock submit <slug>` | Commit the dock's work; `--target github` pushes it |
| `agentdock merge <slug>` | Fold the dock's branch back (direct or PR) |
| `agentdock version` | Print the version |

## Project layout

```
agent/          the agent half — instructions your coding agent reads
  AGENT.md      identity + the one rule + skill index
  REGISTRATION.md
  knowledge/    principles, pitfalls
  skills/       bootstrap · develop · diagnose · reclaim · finish
cmd/agentdock/  the engine entry point
internal/       engine implementation (cli, config, discover, dock, flow, ops, ports, lifecycle, state)
docs/           SPEC and architecture decision records
```

## Status

Core loop (discover / init / new / up / down / clean) is in active development.
Code-lifecycle commands (submit / merge / reclaim) and ops helpers (diagnose /
capacity / gc) are planned for v2. See [docs/SPEC.md](docs/SPEC.md).

## License

MIT
