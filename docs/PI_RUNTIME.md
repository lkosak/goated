# Pi Runtime

Goated supports [Pi](https://github.com/badlogic/pi-mono) as an agent runtime,
which lets you run the agent against any OpenAI-compatible LLM provider
(Fireworks, OpenRouter, Together, Groq, etc.) instead of the built-in Claude
Code or Codex runtimes.

## Status

Pi runtime is **headless-only** and considered working but still maturing. It
has full feature parity for message handling and session resume, but does not
yet expose context estimate or compaction. If that matters to you, stay on
`claude` or `codex`.

## Prerequisites

1. A working `pi` CLI on `PATH`. Install with:
   ```bash
   npm install -g @mariozechner/pi-coding-agent
   pi --version   # should report a version
   ```
2. An API key for at least one OpenAI-compatible provider you want to use.

## One-shot setup

From the Goated repo root, run:

```bash
./goated runtime switch pi
./goated runtime pi configure
./goated daemon restart --reason "pi runtime configured"
```

`runtime pi configure` is an interactive wizard that:

- Prompts for provider name, base URL, API style, API key, model ID, display
  name, context window, and max output tokens.
- Writes (or merges into) `~/.pi/agent/models.json` with `0600` permissions.
- Pins `pi.provider` and `pi.model` in `goated.json` so the daemon passes
  `--provider` and `--model` to every `pi` invocation.
- Runs a real `pi --list-models` probe to verify the provider/model are
  visible before exiting.

Known defaults the wizard will suggest for common providers:

| Provider   | Default base URL                              |
|------------|------------------------------------------------|
| fireworks  | `https://api.fireworks.ai/inference/v1`        |
| openrouter | `https://openrouter.ai/api/v1`                 |
| together   | `https://api.together.xyz/v1`                  |
| groq       | `https://api.groq.com/openai/v1`               |

## Where things live

- `~/.pi/agent/models.json` — Pi's provider/model registry. Written by
  `./goated runtime pi configure`. Contains API keys in plaintext (0600).
- `~/.pi/agent/auth.json` — Pi's persisted OAuth/API-key store for
  **built-in** providers (Claude Pro, ChatGPT, Gemini CLI). Use `pi` then
  `/login` interactively to populate this.
- `goated.json` — Goated config. The `pi.provider` and `pi.model` keys pin the
  runtime so Pi doesn't silently pick a different provider.
- `logs/pi_session/sessions/<id>/` — per-session Pi state directory, keyed by
  a Goated-managed session ID persisted in bbolt under
  `meta["runtime.pi.active_session_id"]`.
- `logs/pi_session/runs/*.jsonl` — one file per Pi invocation, captured from
  stdout/stderr.

## Health and readiness

`./goated runtime status` now does a real probe:

- Checks that `pi` is on `PATH`.
- Runs `pi --list-models` and verifies the configured provider (and model, if
  pinned) is present in the output.
- Reports `Readiness: OK` / `Health: OK` only if the probe succeeds.

If the probe fails you will see a specific error, not a generic "not ready":

```
Readiness: NOT READY (configured pi provider "fireworks" not found in pi --list-models output)
```

## How messages flow

1. Gateway receives a user message, builds a pydict envelope.
2. Pi runtime prepends a **delivery preamble** to the envelope that tells the
   model to execute the `respond_with` shell command rather than answering
   inline. This is the contract that makes Slack/Telegram replies actually
   arrive. See `internal/pi/session.go:piDeliveryPreamble`.
3. Pi is invoked as `pi -p <prompt> -c --mode json --session-dir <per-session dir> --provider X --model Y`.
4. The model writes its reply, shells out to `./goat send_user_message --chat …`, and exits.
5. Goated's daemon socket handler delivers the reply to the channel.

On a fresh session, Goated first runs a **warm-up turn** that tells Pi to read
`GOATED.md` so the runtime contract is loaded before any real message hits.

## /clear behavior

`/clear` in a chat rotates the Goated-managed Pi session ID, wipes the old
per-session directory, and re-runs the warm-up on the next message. This is
the only way to reset Pi's conversation state — `./goated session send /clear`
is a no-op for Pi.

## Common issues

- **"pi binary not found on PATH"** — `npm install -g @mariozechner/pi-coding-agent`.
- **"No API key found for unknown"** — Pi has no provider configured. Run
  `./goated runtime pi configure`.
- **Agent replies but Slack gets nothing** — Pi generated text inline instead
  of shelling out. Check `logs/pi_session/runs/*.jsonl` for the latest run. The
  delivery preamble should prevent this; if it's happening, the preamble may
  need tightening for your chosen model.
- **Identity leak ("I'm Claude")** — Pi/Kimi reading a workspace startup doc
  that mentions Claude/Codex. Check `workspace/AGENTS.md`, `workspace/GOATED.md`,
  `workspace/self/IDENTITY.md` and neutralize the wording.
- **Wrong provider being used** — Set `pi.provider` and `pi.model` in
  `goated.json` (the configure wizard does this for you).
