---
title: Missions System
kind: mission_index
---

# MISSIONS

This folder is for operational work that unfolds over time.

Each mission is a subfolder. A mission folder should contain markdown files with
YAML frontmatter so the state is machine-readable and easy to inspect in
Obsidian-style tools.

## Mission layout

Each mission should have at least:

- `MISSION.md` or `README.md` — mission definition
- `MISSION_LOG.md` — detailed execution log
- `MISSION_TODO.md` — open tasks, blockers, and next actions

## Suggested frontmatter for `MISSION.md`

```yaml
---
title: Mission Name
status: active
priority: medium
goal: One sentence explaining what success looks like
created_at: 2026-03-19
last_advanced_at: 2026-03-19T00:00:00Z
next_action: The single best next step
blockers: []
---
```

## Status meanings

- `active` — should be advanced during heartbeat if possible
- `blocked` — cannot advance without some dependency or decision
- `done` — completed
- `archived` — no longer active, kept for history

## Operating rules

- `MISSION_LOG.md` is the detailed timeline of what happened.
- `MISSION_TODO.md` is the source of truth for what still needs doing.
- Durable facts discovered while doing mission work should be promoted to
  `VAULT/`.
- If you learn something about the user or yourself while doing mission work,
  update `USER.md`, `IDENTITY.md`, `MEMORY.md`, or `SOUL.md` in the same loop.
- Missions should stay concrete. If a mission grows too broad, split it.

## Default mission

Fresh self repos start with `ONBOARD_USER/` as the first active mission.
Complete that mission before expanding into custom missions.
