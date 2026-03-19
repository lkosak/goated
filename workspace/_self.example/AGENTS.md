# AGENTS.md

This directory is a reusable example of a private `self/` repo for Goated.

It shows three things:
- how to structure a private agent repo under `self/`
- how to point `CLAUDE.md` and `GEMINI.md` at one shared instruction file
- how to build a generalized version of the `alan` CLI as a Go/Cobra tool

## Layout

- `AGENTS.md` is the shared entrypoint for agent-specific instructions
- `CLAUDE.md` is a symlink to `AGENTS.md`
- `GEMINI.md` is a symlink to `AGENTS.md`
- `tools/toolbox/` contains a reusable Go CLI skeleton
- `tools/toolbox` is the binary produced by that module after build

## Conventions

- Treat this directory like a private repo mounted inside `workspace/`
- Keep personal state inside this repo, not in the shared workspace root
- Build custom tools as Go binaries that run from `self/`
- Read credentials through `workspace/goat creds get KEY`

## Example CLI

The example CLI under `tools/toolbox/` is named `toolbox`. It generalizes the main
patterns from `alan`:
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
