# ADR 0003: The agent drives; the engine perceives, executes, verifies.

## Status
Accepted.

## Context
AgentDock is "agent-driven", yet `discover` (find the compose stack, list
services/ports) is a deterministic binary. This looks like it leaves nothing
for the agent — if the binary does the "finding", what does the agent do?

Two facts answer this:

- An LLM reading the Aisystem compose YAML silently dropped 443 and 3001.
  LLMs are unreliable at *exhaustive* extraction — they miss, truncate, and
  hallucinate.
- Code has no business judgement: it cannot decide a port-allocation strategy,
  notice that "4 expected services" is actually 6, or ask the human to confirm
  an anomaly.

## Decision
Split work by *capability*, not by "who does the work":

- **Engine (binary) — perception, execution, verification**
  - `discover` reads facts exhaustively (services / ports / required env).
  - `init` / `new` / `up` / `down` / `clean` act deterministically.
  - fail-closed: `new` / `up` reject placeholders and conflicts.
- **Agent — judgement, decision, confirmation, drive**
  - initiate (when and what to isolate),
  - cross-check `discover` output against business understanding,
  - decide (port allocation, naming, boundaries),
  - confirm with the human before acting,
  - drive the engine and interpret its refusals.

`discover` is a *tool the agent uses*, not a replacement for the agent — the way
a scan gives a doctor facts but does not diagnose or prescribe.

## Consequences
- Exhaustive extraction is reliable: no missed services or ports.
- Judgement stays where the context lives (agent + human), not in code.
- A standing test for every future feature: *exhaustive / deterministic → code;
  requires context / judgement / confirmation → agent.*
- The contract stays: the agent proposes, the engine verifies.
