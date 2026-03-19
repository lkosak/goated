---
title: Self Repo Guide
kind: self_instructions
---

# AGENTS.md

This directory is a reusable example of a private `self/` repo for Goated.

It shows three things:
- how to structure a private agent repo under `self/`
- how to point `CLAUDE.md` and `GEMINI.md` at one shared instruction file
- how to build a reusable personal CLI as a Go/Cobra tool
- how to run recurring mission and knowledge maintenance from files in the repo

## Layout

- `AGENTS.md` is the shared entrypoint for agent-specific instructions
- `CLAUDE.md` is a symlink to `AGENTS.md`
- `GEMINI.md` is a symlink to `AGENTS.md`
- `IDENTITY.md` holds your stable identity and voice
- `USER.md` holds durable information about your human
- `MEMORY.md` holds durable working memory
- `SOUL.md` holds your values and deeper self-concept
- `tools/toolbox-cli/` contains a reusable Go CLI skeleton
- `tools/toolbox` is the binary produced by that module after build
- `MISSIONS/` holds operational mission state
- `VAULT/` holds durable knowledge in an Obsidian-style vault
- `HEARTBEAT.md` is the default hourly operational loop
- `prompts/` contains recurring maintenance prompts

## Conventions

- Treat this directory like a private repo mounted inside `workspace/`
- Keep personal state inside this repo, not in the shared workspace root
- Build custom tools as Go binaries that run from `self/`
- Read credentials through `workspace/goat creds get KEY`
- Keep mission execution state in `MISSIONS/`
- Keep durable knowledge in `VAULT/`
- Every markdown file in this repo should start with YAML frontmatter using the
  `---` convention
- If you learn something new about your identity or your user, update the right
  markdown file immediately in the same processing loop so the fact does not
  disappear during later session compaction

## Memory discipline

- Put stable facts about yourself in `IDENTITY.md`
- Put stable facts about the user in `USER.md`
- Put enduring working memory in `MEMORY.md`
- Put values and voice in `SOUL.md`
- Put operational state and next actions in `MISSIONS/`
- Put durable entity knowledge in `VAULT/`

Do not leave important identity or user facts only in chat history. Write them
into the right file as soon as you learn them.

## Default operating system

This example self repo comes with:
- an hourly `HEARTBEAT.md`
- a mission system under `MISSIONS/`
- a durable knowledge vault under `VAULT/`
- a recurring knowledge extraction prompt under `prompts/`

The intent is that a freshly bootstrapped self repo is immediately capable of:
- advancing active missions
- capturing durable knowledge
- reconciling open loops on a schedule

## Example CLI

The example CLI under `tools/toolbox-cli/` is named `toolbox`. It demonstrates the main
patterns for a reusable personal CLI:
- one binary with many subcommands
- automatic `chdir` into the private self repo
- file-backed logs under `logs/`
- local state under `state/`
- credentials fetched at runtime through `goat`

Included commands:
- `toolbox remember` for filesystem-based memory search
- `toolbox browser` for Browser Use automation
- `toolbox voice` for fish.audio TTS
- `toolbox email` for a single `@agentmail.to` inbox
- `toolbox notes` as a proxy to the bundled `notesmd` CLI
- `toolbox creds get` for inspecting configured credentials

## Credentials

This example expects credentials to be managed by `workspace/goat`.

Common setup:

```bash
./goat creds set AGENTMAIL_API_KEY your-agentmail-api-key
./goat creds set AGENTMAIL_INBOX yourname@agentmail.to
./goat creds set BROWSER_USE_API_KEY your-browser-use-api-key
./goat creds set FISH_AUDIO_API_KEY your-fish-audio-api-key
./goat creds set FISH_AUDIO_VOICE_ID your-fish-voice-id
```

Read them back with:

```bash
./goat creds get AGENTMAIL_INBOX
./goat creds get FISH_AUDIO_VOICE_ID
```

Build it from this directory with:

```bash
./build_clis.sh
```

That produces:
- `tools/toolbox`
- `tools/notesmd`

`toolbox` resolves its own location and operates relative to this example self
repo. `toolbox notes ...` proxies to `tools/notesmd`.
