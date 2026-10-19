---
name: develop
description: Open a dock for a task (new + up), work inside it, and keep it isolated. Use when the user asks to work on a task in an isolated environment.
---

# Skill: Develop in a dock

Goal: every task runs in its own dock — its own worktree, port range, and
stack — so parallel tasks never collide.

## 1. Before anything

- Confirm the config is discovered: `.agentdock/config.json` must exist and be
  non-empty. If it is missing or a placeholder, stop and run the `bootstrap`
  skill — do **not** proceed with a guessed list.
- Confirm the dock name (`<slug>`) with the human once.
- If `config.auth.detected` is true, the project has admin login. Ask the human
  how to set this dock's admin username/password **before** `new`. Never invent
  credentials.

## 2. Open the dock

```sh
agentdock new --admin-user <user> --admin-password <password> <slug>
```

or, only if the human explicitly accepts the default:

```sh
agentdock new --use-default-auth <slug>   # admin / admin&798Ad
```

The engine creates the worktree, assigns a port range from its slot model, and
writes the override (container names `songji-<slug>-*`) plus this dock's own
`.env` — a copy of the repo's, with the admin username/password keys overridden
by the values above. If `config.auth.detected` is true and you pass no
credentials, the engine refuses — surface that refusal, do not work around it.

## 3. Start the stack

```sh
agentdock up
```

The engine verifies ports and variables first and refuses rather than starting
a broken stack. 80/443 are never used for a dock.

## 4. Work

Work in the dock's worktree. Never edit the main checkout. If a port is
unexpectedly taken, do **not** pick another by hand — ask the engine
(`agentdock new`/slot model) so the ledger stays truthful.

## 5. Tear down

When finished, run `agentdock down` then `agentdock clean` (see the `reclaim`
skill). Never leave a dock running "just in case" — reclaim it.
