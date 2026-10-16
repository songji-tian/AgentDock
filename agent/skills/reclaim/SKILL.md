---
name: reclaim
description: Tear a finished dock down cleanly and give its ports/worktree back. Use when a task is done or abandoned.
---

# Skill: Reclaim a dock

Goal: leave nothing running and nothing reserved that is not in use.

## 1. Stop, then clean

```sh
agentdock down
agentdock clean
```

`clean` refuses if the stack is still running — stop first. That refusal is a
feature, not a nuisance.

## 2. Verify nothing remains

- No containers with the dock's name prefix (`songji-<slug>-*`).
- No ports reserved in the ledger.
- The worktree is removed.

If anything remains, run `diagnose` before trying again.
