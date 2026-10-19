---
name: reclaim
description: Tear a finished dock down cleanly and give its ports/worktree back. Use when a task is done or abandoned.
---

# Skill: Reclaim a dock

Goal: leave nothing running and nothing reserved that is not in use.

## 1. One-shot reclaim

```sh
agentdock reclaim SLUG
```

Stops the dock (if still up) and then cleans it in one shot. `clean` alone
refuses while the stack is still running — that refusal is a feature, not a
nuisance.

## 2. Batch garbage collection

```sh
agentdock gc          # list garbage docks (dry run, safe)
agentdock gc --yes    # actually reclaim them
```

`gc` only reclaims docks it can prove are not running. Anything it cannot
verify (docker unavailable, no compose files) is left alone and reported
separately — never destroyed on a guess.

## 3. Verify nothing remains

- `agentdock capacity` — the ledger no longer reserves the dock's ports.
- `agentdock diagnose SLUG` — should report the dock is gone.
- No containers with the dock's name prefix (`<project>-<slug>-*`).

If anything remains, read the `diagnose` output before trying again.
