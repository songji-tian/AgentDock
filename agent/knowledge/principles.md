# Principles

- **Agent-first.** The product is an agent; the engine is its hands.
- **Never guess. Verify, or refuse.** Missing facts are discovered, not assumed.
- **Empty init.** The engine writes no default service list; the list always
  comes from `discover` + human confirmation.
- **Deterministic over clever.** Ports come from a slot model, not random picks.
- **Fail-closed.** The engine refuses incomplete or unsafe requests.
- **One confirmation, then automation.** Confirm the service/port list and dock
  name once; derive the rest.
