---
name: finish
description: Finish a task: deliver the dock's work (submit), fold it back (merge), then reclaim. Use when the work in a dock is done.
---

# Skill: Finish a task

Goal: deliver the work done in a dock, then clean up — nothing left running,
nothing left reserved.

## 0. Confirm the finish environment first (agent-driven)

Before `submit`, confirm with the user where the work goes:

- **local** — commit only, no push (the default).
- **personal GitHub** — push to the user's own GitHub. **Private by default**;
  pass `--visibility public` to make it public. A missing repo is created
  automatically via `gh repo create`.
- Corporate GitLab / cloud / open-source GitHub — **not wired yet**; confirm
  with the user before attempting anything there.

Never guess the environment. Also keep intermediate artifacts (like discover
JSON dumps) **outside** the repo directory — a stray untracked file makes the
main checkout dirty and blocks `merge --mode direct`.

## 1. Deliver

```sh
agentdock submit SLUG                        # commit only (local)
agentdock submit --target github SLUG        # commit + push (private)
agentdock submit --target github --visibility public SLUG
```

`submit` refuses on nothing-to-commit or a missing worktree.

## 2. Fold back

```sh
agentdock merge --mode direct SLUG           # merge into main locally
agentdock merge --mode pr SLUG               # open a PR (branch must be pushed)
```

`direct` refuses on a dirty main checkout, on the wrong branch, or on a
conflict — conflicts are resolved by hand, never auto-merged.

## 3. Reclaim

```sh
agentdock reclaim SLUG                       # stop + clean in one shot
```

See the `reclaim` skill for details and for `gc`.
