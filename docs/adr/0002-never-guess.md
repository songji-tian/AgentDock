# ADR 0002: Never guess. Verify, or refuse.

## Status
Accepted.

## Context
The previous tool's `init` wrote a hardcoded service list (api/web/db/redis)
without reading the repository. When a project had other services (an nginx
443, a webhook on 3001), the list was silently wrong and the stack failed.
This violated the tool's own "never guess" rule.

## Decision
The engine never guesses:

- `init` writes nothing on its own; it accepts a confirmed fact list via
  `init --from <facts>`.
- `new`/`up` refuse when the config is absent, a placeholder, or incomplete.
- Discovery is done by `agentdock discover` (a deterministic parser), not by an
  LLM reading YAML.

## Consequences
- A missing service/port is caught at discovery or at `up`, loudly, instead of
  producing a broken stack.
- The split of responsibilities is: the agent proposes, the engine verifies.
