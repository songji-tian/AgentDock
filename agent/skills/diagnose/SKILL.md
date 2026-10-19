---
name: diagnose
description: Read state and logs before retrying when a dock misbehaves. Use when new/up/down/clean fails or a service is unreachable.
---

# Skill: Diagnose a dock

Goal: turn a failure into facts before acting. Never retry blind.

## 1. Read the refusal first

`agentdock` refuses with a specific message. Read it. Most failures are the
engine correctly refusing an incomplete or unsafe request — the fix is to
complete the request, not to bypass the engine.

## 2. Run the read-only checks

```sh
agentdock capacity          # port ledger + free space
agentdock diagnose [SLUG]   # per-dock consistency (read-only, never mutates)
```

`diagnose` reports `ok` / `warn` / `err` findings: worktree presence, compose
override presence, `.env` presence, ledger status. Treat `err` as a blocker;
`warn` means "will be fixed on the next `up`".

## 3. Read state, then logs

- `.agentdock/state.json` — what ports/slug/worktree this dock owns.
- `docker compose ps` / `docker ps` — what is actually running.
- `docker compose logs <service>` — why a service did not come up.

## 4. Verify before changing anything

- Is the port actually free? (Ask the engine, do not guess.)
- Is the required variable set? (`${VAR:?...}` will say which one.)
- Is a container name colliding with the main stack?

Report findings with evidence. Ask one focused question if you need a decision.
