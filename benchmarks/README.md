# GPTCode agent evaluation

The evaluation harness runs coding agents against the same clean Git fixture
and records an inspectable evidence bundle. It is designed to answer a narrow
question: did the agent produce a change that satisfies executable,
task-specific checks without losing the initial repository state?

An experiment configuration names the agent process explicitly. Commands are
executed directly, without an implicit shell. The evidence directory must live
outside the evaluated repository so the run cannot accidentally measure its
own artifacts.

```json
{
  "id": "go-ledger-deadlock",
  "repository": "/tmp/go-ledger",
  "output": "/tmp/go-ledger-evidence",
  "agent": {
    "name": "gptcode-local",
    "args": [
      "/Users/jadercorrea/bin/gt",
      "do",
      "Fix the opposing-transfer deadlock without changing the public API"
    ]
  },
  "verifications": [
    {
      "name": "tests-and-race-detector",
      "args": ["go", "test", "-race", "./..."]
    },
    {
      "name": "static-analysis",
      "args": ["go", "vet", "./..."]
    }
  ]
}
```

Run it with:

```bash
go run ./scripts/evidence-run -config /tmp/experiment.json
```

Every result retains failures as evidence. A successful agent process does not
make a run pass: every verification command must also exit successfully.

## Repeatability suites

The suite runner creates a fresh Git repository for every fixture repetition
and aggregates all outcomes. It runs sequentially so local CPU and memory
contention do not bias model comparisons.

```bash
go run ./scripts/evidence-suite \
  -verbose \
  -config benchmarks/suites/local-gpt-oss.json \
  -output /tmp/gptcode-go-core-quality
```

`-verbose` streams the agent's inspectable stages while retaining identical
stdout and stderr in the evidence bundle. It exposes model selection, planning,
tool execution, retries, and deterministic checks; it does not print private
model chain-of-thought.

Use `benchmarks/suites/local-qwen3-coder.json` for the equivalent Qwen suite.
Each committed local suite records and executes its required GPTCode profile
selection in `setup.json`; the agent name is not merely a user-supplied label.
`benchmarks/suites/local-qwen3-coder-cache.json` preserves the first
human-reviewed three-run result on the strengthened cache contract.
`benchmarks/suites/local-devstral-small-2-safe-store.json` records the
hardware-tuned 16k Devstral configuration that produced the first reviewed
pass on strengthened filesystem containment.

The initial corpus covers three distinct failure classes:

- lock ordering and deterministic concurrency;
- TTL boundary semantics under concurrent cache access;
- path traversal and symlink containment at a filesystem boundary.

The committed suite uses three repetitions. During harness development,
`-repetitions 1` provides a smoke test without presenting it as a consistency
measurement. `run_timeout_seconds` is a hard agent budget; verification still
runs after a timeout so the resulting repository state remains evidence.
Fixtures with `require_failing_baseline` also write `baseline.json` and abort
the suite if every check already passes before the agent runs.

Development results are recorded under `benchmarks/results/`. They distinguish
system defects from model failures and never promote a one-run smoke test to a
repeatability claim.

## Bundle contract

The generated directory contains:

- `manifest.json`: schema, base commit, timestamps, and snapshot hashes.
- `initial.tar` and `final.tar`: deterministic repository snapshots excluding
  Git metadata.
- `initial.patch`, `agent.patch`, and `final.patch`: binary-capable Git diffs.
- `events.jsonl`: lifecycle events.
- `commands.jsonl`: exact commands, outputs, exit codes, and durations.
- `verification.json`: the machine-readable pass/fail decision.
- `report.md`: a compact human-readable summary.

Snapshots are intentionally complete. Only use controlled public fixtures:
real repositories may contain credentials or proprietary untracked files.
