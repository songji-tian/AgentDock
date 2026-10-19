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
agentdock discover --json
```

This parses the repository's compose file(s) and prints a structured fact list:
every service, its ports (host→container), its `container_name` (if hardcoded),
every required variable (`${VAR:?...}` / `env_file`), and `auth_signals` — soft
hints that the project has account login.

Do **not** read the compose with your own eyes and summarise it. Use
`agentdock discover` — the whole point is that a deterministic parser does not
miss a port the way a reading of YAML can.

## 2. Detect admin login (if any)

If `auth_signals` is non-empty, the project likely has account login. Confirm it
by reading the project (do not rely on the signal alone):

- which env keys hold the admin username / password (e.g. `ADMIN_USERNAME`,
  `ADMIN_PASSWORD`),
- which seed script / management command creates the superuser.

Confirm with the human once, then record it in `.agentdock/config.json` under
`auth`:

```json
"auth": {
  "detected": true,
  "username_env": "ADMIN_USERNAME",
  "password_env": "ADMIN_PASSWORD",
  "seed_hint": "manage.py createsuperuser"
}
```

If there is no login, leave `auth` absent (or `detected: false`). The engine
uses this to refuse a dock that would start with no admin credentials.

## 3. Confirm with the human (once)

Show the discovered list **once** and ask for confirmation before writing
anything. Cover at least:

- the number of ports and which service each belongs to
- whether 80/443 or any other port must **not** be used by a dock
- the container-name convention (e.g. `songji-<slug>-*`)
- the required variables and where they come from
- the admin-login finding from step 2 (if any)

Ask **one** focused question, not a wall of text.

## 4. Write the config

After confirmation, write the confirmed list into `.agentdock/config.json` via:

```sh
agentdock init --from <facts>
```

`agentdock init` on its own writes nothing — it is empty by design. The service
list always comes from `--from`, never from a built-in default. If login was
detected (step 2), record the `auth` block in the config as well.

## 5. Verify

Prove it works before declaring success: open a dock (`agentdock new` — see the
`develop` skill for the admin-credentials flags), start it (`agentdock up`),
confirm the services are reachable on the assigned ports and that 80/443 were
not used, then tear it down (`agentdock clean`). If any step refuses, read the
refusal and fix the config — do not work around it.
