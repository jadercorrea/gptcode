# Local GPT-OSS smoke evaluation — 2026-07-28

This is a development smoke test, not a benchmark claim. Each fixture ran once;
repeatability requires the three repetitions declared by the suite.

## Corpus

| Fixture | Failure class | Deterministic contract |
| --- | --- | --- |
| `ledger-deadlock` | lock ordering and self-deadlock | tests, race detector, formatting, no `unsafe`, vet |
| `cache-expiration` | TTL boundary and stale eviction | tests, race detector, formatting, vet |
| `safe-store` | traversal and symlink containment | tests, race detector, formatting, vet |

## First smoke

The first run exposed harness and CLI defects before it could measure the model
fairly:

- a planner stage escaped the local profile and attempted OpenRouter;
- structured `required_files` output could not be decoded;
- Go formatting remained probabilistic.

Result: **0/3 passed**, median duration **1m57s**. These runs remain preserved
locally, but are not treated as model-performance evidence.

## Corrected smoke

The CLI was rebuilt with:

- a hard prohibition on cloud backends in local mode;
- support for string and structured movement file references;
- deterministic `gofmt` on modified Go files before verification.

| Fixture | Result | Duration | Final failure |
| --- | ---: | ---: | --- |
| `ledger-deadlock` | failed | 7m38s | invalid imports and unresolved `unsafe` reference |
| `cache-expiration` | failed | 5m50s | `Len` still counted the expired entry |
| `safe-store` | failed | 5m31s | malformed final source after retries |

Corrected result: **0/3 passed**, median duration **5m50s**.

No corrected run contacted OpenRouter. The model identified each root cause and
made relevant edits, but did not converge to a verified implementation within
three attempts. The current evidence therefore supports local exercisability,
not acceptable coding-agent reliability.

## Next measurement

Do not publish a comparative success-rate claim yet. First:

1. enforce the ten-minute per-run budget in the suite;
2. test a faster local model on the same corpus;
3. run all three repetitions only for configurations that pass at least one
   smoke fixture;
4. publish every run, including failures and timeouts.

## Qwen3-Coder comparison

The same corrected 1×3 smoke was run with `qwen3-coder:latest`.

| Fixture | Initial result | Duration |
| --- | ---: | ---: |
| `ledger-deadlock` | failed | 1m38s |
| `cache-expiration` | failed | 1m41s |
| `safe-store` | passed contract | 1m51s |

Initial aggregate: **1/3 passed**, median **1m41s**. This was substantially
faster than GPT-OSS, but human review found that the safe-store implementation
rejected every name containing `..`, including the legitimate
`reports/v1..v2.txt`.

The contract was strengthened to preserve dots inside ordinary path segments.
A new Qwen safe-store run then failed in **1m35s** after three attempts.
Therefore the earlier pass is classified as a contract false positive, not a
quality-confirmed success.

That conclusion changed after closing the editor feedback loop.

## Grounded closed-loop Qwen result

The CLI previously stopped the editor as soon as every planned file had been
touched. It also omitted tests and `task.md` from planning and retry context.
The corrected loop now:

- grounds the initial plan and retries in implementation, tests, and task
  constraints;
- lets the editor run verification and repair the same file within one turn;
- permits up to twenty tool calls for inspect → edit → verify → repair;
- rejects empty local-model answers and propagates Ollama HTTP errors.

Human review found two more fixture weaknesses before accepting a result:

- safe-store traversal appeared to pass only because `/var` is a symlink on
  macOS; the strengthened test resolves the temporary root first;
- cache `Len` was not required to purge expired entries unless `Get` had
  already been called; the strengthened test now checks independent cleanup.

Qwen did not solve the strengthened safe-store contract within the ten-minute
budget. It did solve the strengthened cache contract, and three clean
repetitions produced:

| Repetition | Result | Duration |
| --- | ---: | ---: |
| 1 | passed | 4m07s |
| 2 | passed | 3m56s |
| 3 | passed | 4m46s |

Reviewed aggregate: **3/3 passed**, median **4m07s**, with `go test -race ./...`
and `go vet ./...` passing after every agent run. The three patches used
different locking strategies but all preserved the exported API, treated the
exact TTL boundary as expired, and purged stale entries through both `Get` and
`Len`.

This is evidence of repeatability for one bounded temporal/concurrency task,
not a general coding-agent success-rate claim. Filesystem containment remains
an observed failure class.
