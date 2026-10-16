# Pitfalls

- **Do not summarise the compose from memory/eyes** — use `agentdock discover`.
  An LLM reading YAML misses ports (this is how 443 and a webhook port were
  missed once). The parser does not miss.
- **Never use 80/443 for a dev dock.** The main stack owns them. A dock that
  touches them collides with the main stack.
- **Hardcoded `container_name` overrides the project prefix.** The engine must
  rewrite them per-dock (`songji-<slug>-*`), not trust compose defaults.
- **Required variables (`${VAR:?...}`) block startup at the first compose call.**
  Supply them before `up`, or `up` fails with a misleading missing-key error.
- **Do not hand-pick ports.** It desynchronises the ledger; the next dock then
  collides.
