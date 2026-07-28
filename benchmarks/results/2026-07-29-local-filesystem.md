# Local filesystem-containment evaluation — 2026-07-29

This development evaluation compares local configurations on the strengthened
`safe-store` fixture. It is a single-run capability check, not a repeatability
or general success-rate claim.

## Hardware and execution

- Apple M1 Pro, 16 GPU cores, 32 GB unified memory.
- Ollama served every measured inference locally.
- `ollama ps` reported Qwen at 24 GB and **100% GPU** with a 65,536-token
  context.
- Devstral Small 2 used 25 GB at 65,536 tokens and offloaded 8% to CPU.
- Setting `GPTCODE_OLLAMA_CONTEXT_LENGTH=16384` reduced Devstral to 17 GB and
  kept it at **100% GPU**.

Kimi was not evaluated locally. Current Kimi releases do not fit this hardware
as a comparable fully local coding-agent configuration.

## Adaptive retry budget

The CLI now stops after three equivalent deterministic-verification failures.
Timing noise is normalized before comparison, while a changed failure resets
the counter. This permits larger local-model budgets without spending every
attempt on a demonstrated plateau.

Qwen received 30 minutes and eight allowed attempts. It stopped after six
attempts in **16m27s** when the symlink-parent failure repeated three times.
More time did not resolve the strengthened contract.

## Devstral calibration

The first Devstral run exposed a fixed 120-second Ollama client timeout before
the model could edit. The provider now defaults to ten minutes per request and
supports:

- `GPTCODE_OLLAMA_TIMEOUT`, parsed as a Go duration;
- `GPTCODE_OLLAMA_CONTEXT_LENGTH`, sent to Ollama as `num_ctx`.

At 32k, Devstral remained fully GPU-backed but timed out at 30 minutes after
repeating an incorrect `EvalSymlinks` strategy and entering a no-diff tool
loop.

At 16k, Devstral completed in **26m08s**:

| Gate | Result |
| --- | ---: |
| Agent exit | passed |
| `go test -count=1 -race ./...` | passed |
| `go vet ./...` | passed |
| Files modified | 1 |
| API cost | $0 |

The first edit failed only the symlinked-parent test. The second edit passed
the deterministic contract. The evidence suite streamed the verbose execution
while preserving the same stdout and stderr in `commands.jsonl`.

## Human review

The Devstral patch correctly rejects the fixture's traversal, absolute-path,
and pre-existing symlink cases while permitting nested paths and ordinary dots
inside path segments.

It is not presented as a general production filesystem primitive. Its
symlink check and subsequent write are separate operations, leaving a TOCTOU
window under a concurrently hostile filesystem. The result proves the bounded
fixture contract, not race-free containment against an active attacker.

One reviewed pass justifies further repetition. It does not justify a general
model reliability claim.

## Three-run repeatability result

The same committed 16k configuration was then run three times from clean
fixtures with a 30-minute budget per run.

| Repetition | Result | Duration | Final observed failure |
| --- | ---: | ---: | --- |
| 1 | timed out | 30m01s | legitimate nested paths rejected |
| 2 | timed out | 30m01s | legitimate nested paths rejected |
| 3 | timed out | 30m01s | absolute path and symlink escape accepted |

Repeatability result: **0/3 passed**, median **30m01s**.

All three runs remained fully local and GPU-backed. They produced relevant
edits, but did not converge before the task budget. The earlier reviewed pass
remains evidence that this configuration can solve the bounded contract; the
repeatability suite is stronger evidence that it cannot yet do so reliably.

The runs also exposed nondeterministic Go temporary-directory IDs in otherwise
equivalent test failures. The plateau detector now normalizes those paths so
future local runs do not mistake ephemeral filesystem names for engineering
progress.
