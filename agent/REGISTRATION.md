# Registering AgentDock with your coding agent

AgentDock is **not a library you import** — it is an *agent* you install into
your coding agent's context. "Registering" means making your runtime aware of
`agent/AGENT.md` (identity + rules) and `agent/skills/` (procedures).

## The idea

```
your coding agent (Codex / Claude Code / Cursor / OpenCode / …)
        │  reads
        ▼
  agent/AGENT.md      ← who it is, the one rule, the skill index
  agent/skills/*      ← bootstrap · develop · diagnose · reclaim · finish
        │  drives
        ▼
  agentdock           ← the deterministic engine (the agent's "hands")
```

The agent brings judgement; the engine brings guarantees. Keep them separate.

## The universal method (works everywhere)

1. Install the engine and put it on `PATH`:

   ```sh
   go install github.com/songji-tian/agentdock/cmd/agentdock@latest
   # or: download a release binary and copy it to /usr/local/bin/agentdock
   agentdock version
   ```

2. Keep this repository (or a clone of it) somewhere stable, e.g.
   `~/.agentdock/`.

3. Run `agentdock register` in your project — it writes a managed pointer for
   you:

   ```sh
   agentdock register --dir /path/to/your/project
   agentdock register --dir /path/to/your/project --check   # verify (exit 1 = not registered)
   ```

   `register` targets `AGENTS.md` (all runtimes that read root instructions)
   and `CLAUDE.md` when it exists (Claude Code). The marker-delimited block is
   idempotent, so re-running is safe; `--remove` deletes it.

   Almost every runtime also supports a vendor format we do not write:

   | Mechanism | File / location | Used by |
   |---|---|---|
   | Root instruction file | `AGENTS.md` in your project | Codex, increasingly others |
   | Project instruction file | `CLAUDE.md` / `.claude/` | Claude Code |
   | Rules directory | `.cursor/rules/*.mdc` | Cursor |
   | Skill directory | `.claude/skills/`, `skills/` | skill-aware runtimes |

   For those, paste the same pointer by hand:

   ```markdown
   Before any worktree, port, environment, or Docker work, read
   ~/.agentdock/agent/AGENT.md and follow its skills.
   ```

4. Verify registration without asking anyone: `agentdock register --check`
   must exit 0. Then ask the agent: *"what is the one rule?"* A registered
   AgentDock answers **"Never guess. Verify, or refuse."**

## Repository-local vs global

- **Global** (`~/.agentdock/`): one install, every project benefits.
- **Repo-local** (vendored under the project): reproducible, reviewable, survives
  a fresh machine. Good for teams.

Vendoring is a copy, not a fork — re-run the installer to update.

## After registering

Ask the agent to run the **bootstrap** skill on your repo. That is the whole
onboarding: `agentdock discover` reads your stack, you confirm the service/port
list once, the agent writes `.agentdock/config.json`, and it proves the result
with a smoke test.

> If your runtime has no "skills" concept, the `SKILL.md` files are still
> readable instructions — paste the relevant one into your prompt. The agent
> contract does not depend on any vendor feature.
