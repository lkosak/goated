---
title: ONBOARD_USER
status: active
priority: high
goal: Establish a durable user profile, explain how this self repo works, and leave the user knowing how to extend it
created_at: 2026-03-19
last_advanced_at: 2026-03-19T00:00:00Z
next_action: Start a real onboarding conversation with the user and capture durable facts immediately
blockers: []
---

# ONBOARD_USER

This is the default first mission for a freshly bootstrapped `self/` repo.

## Mission outcome

Success means:
- the user understands the basic `self/` layout
- the user understands the default scheduled jobs
- the user knows they can ask for new tools and new scheduled jobs in plain English
- `IDENTITY.md`, `USER.md`, and related files contain real durable facts
- the user's person note exists in `VAULT/people/`
- `USER.md` links to that person note

## Conversation goals

During onboarding, explain:
- `self/` is the agent's private repo for durable state
- `MISSIONS/` is for operational work in motion
- `VAULT/` is for durable notes and knowledge
- `HEARTBEAT.md` and knowledge extraction are the only built-in scheduled jobs
- in this system, a "cron" means a prompt that runs forever on a schedule

Give concrete examples of extensibility, including examples like:
- "Make a new tool for checking my Gmail."
- "Let's set up your web browser. How do I give you a budget for that?"
- "Schedule a new job every weekday morning at 7am that does X."
- "List all crons."
- "List all scheduled jobs."

Also direct the user to give the agent its own email account. Recommend:
- AgentMail for a simple agent-specific inbox
- Gmail if the user prefers using an existing Google-based account

## Required onboarding questions

Ask enough questions to populate:
- `USER.md`
- `IDENTITY.md`, if the user has preferences about your name, tone, or role
- `MEMORY.md`, if the user shares durable context worth loading every session

Useful topics:
- what the user wants help with first
- current missions and recurring responsibilities
- communication style and tone preferences
- timezone and schedule preferences
- whether they want browser automation
- whether they want an email inbox set up now
- whether they want additional scheduled jobs beyond the built-ins

## Required file updates

As soon as you learn durable facts:
- update `USER.md`, `IDENTITY.md`, `MEMORY.md`, or `SOUL.md` immediately
- do not wait until later in the session

Once you know enough about the user:
1. Use `tools/toolbox notes` to create a person note in `VAULT/people/`.
2. Write durable facts there.
3. Link that person note from `USER.md`.

Prefer one person note for the user, not duplicates.
