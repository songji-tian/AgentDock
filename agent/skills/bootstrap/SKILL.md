---
name: bootstrap
description: Install AgentDock into a repository that has never used it. Discover the stack, confirm with the human once, write .agentdock/config.json, and verify. Use when the user says "set up AgentDock", "add isolated environments to this repo", or on first run.
---

# Skill: Bootstrap a repository

Goal: turn an ordinary repository into one that AgentDock can open docks for,
with a service list that comes from the repository's real stack — never from a
guess.

## 1. Discover (verify, do not guess)

Run:

```sh
agentdock discover
```

This parses the repository's compose file(s) and prints a structured fact list:
every service, its ports (host→container), its `container_name` (if hardcoded),
and every required variable (`${VAR:?...}` / `env_file`).

Do **not** read the compose with your own eyes and summarise it. Use
`agentdock discover` — the whole point is that a deterministic parser does not
miss a port the way a reading of YAML can.

## 2. Confirm with the human (once)

Show the discovered list **once** and ask for confirmation before writing
anything. Cover at least:

- the number of ports and which service each belongs to
- whether 80/443 or any other port must **not** be used by a dock
- the container-name convention (e.g. `songji-<slug>-*`)
- the required variables and where they come from

Ask **one** focused question, not a wall of text.

## 3. Write the config

After confirmation, write the confirmed list into `.agentdock/config.json` via:

```sh
agentdock init --from <facts>
```

`agentdock init` on its own writes nothing — it is empty by design. The service
list always comes from `--from`, never from a built-in default.

## 4. Verify

Prove it works before declaring success: open a dock (`agentdock new`), start
it (`agentdock up`), confirm the services are reachable on the assigned ports
and that 80/443 were not used, then tear it down (`agentdock clean`). If any
step refuses, read the refusal and fix the config — do not work around it.
