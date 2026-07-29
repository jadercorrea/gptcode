---
layout: post
title: "One Successful Agent Run Proves Almost Nothing"
date: 2026-07-29
author: Jader Correa
series: Evidence-Based AI Engineering
format: Engineering paper
tags: [ai-agents, software-engineering, evaluation, local-models, verification]
description: "A local coding model passed a security-sensitive task once and failed the next three clean runs. What the experiment taught me about capability, reliability, and evidence."
image: /assets/agent-reliability-results.png
---

<aside class="paper-abstract" aria-label="Executive summary">
  <strong>In brief: Capability is not reliability.</strong>
  A local coding model satisfied a security-sensitive contract once in 26
  minutes. In a subsequent three-run repeatability campaign, the same
  configuration timed out every time. The first result demonstrates
  possibility. The later results do not support a reliability claim.
</aside>

A coding agent completed a security-sensitive task on my machine. It changed
one Go file, passed the tests under the race detector, passed static analysis,
and used no paid API.

Then I ran the same experiment three more times.

It failed every run.

This is not an argument that coding agents are useless. It is an argument that
the unit of evidence matters. A successful demonstration establishes that a
system can produce a result. It does not establish how often the result can be
produced, how dependent it is on sampling, or whether the process is suitable
for unattended engineering work.

The distinction is easy to lose because a good agent run is unusually
persuasive. We see the model inspect files, edit code, run tests, repair a
failure, and announce completion. The transcript resembles work. The final
diff may even be correct.

But one run is still one observation.

<figure class="result-figure">
  <img
    src="{{ '/assets/agent-reliability-results.svg' | relative_url }}"
    alt="The capability campaign passed one of one runs. The subsequent repeatability campaign timed out in all three runs. Capability was demonstrated; reliability was not."
    width="1200"
    height="630"
  >
  <figcaption>
    Two sequential campaigns with different purposes: a capability check,
    followed by a controlled repeatability measurement.
  </figcaption>
</figure>

## The task

I was evaluating local execution in
[GPTCode](https://github.com/jadercorrea/gptcode), an open-source coding CLI
built around explicit research, planning, implementation, review, and
verification stages.

The fixture was deliberately small. It contained a Go package with a `Store`
that writes a named file beneath a configured root:

```go
type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Write(name string, content []byte) error {
	path := filepath.Join(s.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}
```

The implementation was vulnerable by construction. A caller could supply an
absolute path, use parent-directory traversal, or escape through a symlinked
parent.

The requested change had to satisfy several properties:

- ordinary nested files must still work;
- dots inside a legitimate filename, such as `v1..v2.txt`, must remain valid;
- direct and normalized traversal must be rejected;
- absolute paths must be rejected;
- a parent directory that is already a symlink must not permit an escape;
- the public API must remain unchanged;
- tests must pass under the race detector;
- `go vet` must pass.

This is a small task in lines of code, but not a trivial one. Filesystem
containment contains edge cases that plausible-looking prefix checks often
miss. It was useful precisely because the contract could distinguish a real
solution from a convenient approximation.

## The contract had already caught a false positive

An earlier version of the fixture appeared to pass with Qwen3-Coder. Human
review found that the generated implementation rejected every name containing
`..`, including the legitimate `reports/v1..v2.txt`.

The agent had not solved path traversal. It had prohibited a substring.

The contract was strengthened.

A second weakness appeared on macOS. The temporary root used by the test could
pass through `/var`, which is itself a symlink. A naive implementation could
therefore appear to reject the malicious case for the wrong reason. The test
was changed to resolve the temporary root before constructing the adversarial
path.

These were not model improvements. They were measurement improvements.

That distinction is important. Better benchmarks often make systems look worse
before they make them better. A falling score can mean the evaluation has
stopped rewarding shortcuts.

## The first Devstral run

I tested Devstral Small 2 through Ollama on an Apple M1 Pro with 32 GB of
unified memory.

The initial configuration used a 65,536-token context. Ollama reported a
25 GB runtime footprint with eight percent of the model offloaded to CPU. A
32,768-token context kept the execution fully on the GPU, but the run exhausted
the 30-minute task budget without converging.

At 16,384 tokens, the model occupied 17 GB and Ollama reported 100 percent GPU
execution. The repository was tiny, so the smaller context still contained
the task, source, tests, skills, and retry evidence.

That run completed in 26 minutes and 8 seconds.

The first implementation passed every test except the symlinked-parent case.
GPTCode returned the exact deterministic failure to the editor. The second
implementation passed:

```text
go test -count=1 -race ./...    PASS
go vet ./...                    PASS
```

The agent modified one file. The exported API remained unchanged. No paid API
was called.

This was a legitimate success against the executable contract. It was also an
attractive demonstration:

```text
Analyze
Plan
Implement
Verify
Repair
Verify
Pass
```

It would have been easy to stop there.

## The repeatability run

Instead, I ran the same committed configuration three times.

Each repetition began with:

- a fresh copy of the fixture;
- a new Git repository and baseline commit;
- a required failing baseline;
- the same model profile;
- the same 16,384-token context;
- the same 30-minute budget;
- the same maximum retry count;
- the same verification commands.

Runs were sequential so local resource contention would not bias the
comparison. Ollama continued to report 100 percent GPU execution.

The result was:

| Repetition | Result | Duration | Final observed failure |
| --- | ---: | ---: | --- |
| 1 | timed out | 30m01s | legitimate nested paths rejected |
| 2 | timed out | 30m01s | legitimate nested paths rejected |
| 3 | timed out | 30m01s | absolute path and symlink escape accepted |

Pass rate: **0/3**.

Median duration: **30m01s**.

The model did not ignore the task. It produced relevant plans and several
substantive implementations. It recognized traversal, absolute paths, and
symlinks. Some attempts came close.

It did not converge within the agreed budget.

In two runs, it tried to validate symlinks by calling `lstat` or
`EvalSymlinks` on a path that did not yet exist. This rejected legitimate
nested writes. In another, it implemented lexical containment but failed to
reject an absolute input and a symlinked parent. Some retries regressed a case
that had passed in the previous attempt. One editor cycle consumed twenty tool
calls without producing a diff.

These are not cosmetic variations. They are different failure modes at the
security boundary.

## Capability is not reliability

The first run and the next three runs are not contradictory.

Together they support a more precise conclusion:

> The configuration demonstrated the capability to solve the bounded task,
> but did not demonstrate the reliability required to solve it repeatedly.

Capability asks whether a result is possible.

Reliability asks how often the result is produced under controlled repetition.

An impressive demo usually answers the first question. Engineering decisions
usually require the second.

This is especially important for agentic systems because the visible run is
often selected after development, prompt adjustment, model selection, and
several discarded attempts. The viewer sees a coherent trajectory. The system
owner remembers the search process. Unless failures are retained, neither can
calculate what the demonstration represents.

## The agent did not fail alone

The experiment also found defects in the surrounding system.

The first Devstral attempt never reached an edit because GPTCode's Ollama
client had a fixed 120-second request timeout. The suite allowed 30 minutes,
but an internal client layer silently imposed a much smaller budget. The
provider now has a longer local default and explicit controls for request
timeout and context length.

The evidence suite originally captured stdout and stderr but displayed them
only after completion. A 20-minute local run therefore looked indistinguishable
from a hung process. It now supports verbose streaming while preserving the
same output in the evidence bundle.

The retry loop could also spend its remaining budget repeating an equivalent
test failure. GPTCode now detects a verification plateau and stops after three
equivalent failures. Timing noise and Go temporary-directory identifiers are
normalized so ephemeral output does not masquerade as progress.

Finally, the execution summary once reported `STATUS: FAILED` when the final
verification had passed, because it treated recoverable intermediate errors
as the authoritative outcome. Intermediate errors are still retained, but the
final status now reflects the verified result.

None of these changes made the model more intelligent.

They made the system more truthful, observable, and economical when the model
was not intelligent enough.

## What should count as evidence

For a coding-agent experiment, I now want at least the following:

1. The initial repository state, including the baseline used by the agent.
2. The exact task and model configuration.
3. A clean workspace for every repetition.
4. The agent's stdout, stderr, exit code, and duration.
5. The patch produced by the agent, including failed runs.
6. Deterministic verification commands and their outputs.
7. A final repository snapshot that can be restored independently.
8. Human review of what the executable contract does not prove.
9. Repetition before making a reliability claim.

GPTCode's evaluation harness now records these as replayable evidence bundles.
A successful agent exit is insufficient. Every required verification command
must also pass.

The inverse matters too: a failed run must not disappear. Failed patches show
whether the model misunderstood the problem, chose an unsafe approximation,
entered a tool loop, or simply ran out of time near a valid solution.

Aggregate scores without failed artifacts conceal most of the engineering
information.

## Tests establish a boundary, not universal correctness

Even the successful run should not be overstated.

The generated implementation passed the fixture's traversal, absolute-path,
and pre-existing-symlink cases. Human review found that its symlink check and
file write were separate operations. A concurrently hostile process could
potentially change the filesystem between the check and the write.

The patch satisfied the declared fixture. It was not a race-free filesystem
primitive suitable for an active adversary.

This is not a reason to dismiss tests. It is a reason to state their boundary.
Executable verification is stronger than the model's assertion, but it proves
the encoded contract, not every property a future environment may require.

Repositories define constraints. Verification establishes truth only within
those constraints. Engineering review still decides whether the constraints
are sufficient.

## From recording studios to local inference

My interest in local models did not begin with a model benchmark. It came from
an older change I watched happen in music.

When I started recording, going to a studio was not merely a preference. The
equipment, room, engineering knowledge, and production infrastructure were
there. If I wanted a serious result, I had to go where those resources were.

Today I record at home. For most of what I need, the tools have become good
enough that hiring a studio is no longer the default. This did not make
professional studios obsolete. A purpose-built room, an experienced engineer,
specialized equipment, and the ability to coordinate a large production still
matter. What changed was the boundary between work that required centralized
infrastructure and work that could be done well on a personal machine.

Voice production and radio went through related changes. Work that once
depended on access to a particular facility can now be produced, edited, and
distributed from a much smaller setup. Large studios and broadcast
infrastructure remain valuable, but they are no longer mandatory for every
recording.

I suspect part of the AI market will move in the same direction.

Frontier-model providers, serverless inference, and managed MLOps will continue
to be necessary. They concentrate hardware, operational expertise, model
access, and reliability that most individuals and companies should not
recreate. But that does not mean every repository question, code review, or
bounded change will always need to be sent to Claude, Codex, or another hosted
system.

This experiment was an attempt to locate that boundary.

In a separate temporal and concurrency fixture, Qwen3-Coder completed three
clean runs with a median of 4 minutes and 7 seconds. Each run passed the race
detector and static analysis. The filesystem experiment produced a different
answer: Devstral demonstrated that it could solve the contract once, then
failed three controlled repetitions.

That is more useful than a general claim that local models are either ready or
useless. Some work has already moved into the home studio. Some work still
needs the larger room.

The engineering problem is to tell the difference before wasting time or
accepting an unreliable result.

## The next system should escalate evidence

When a local model stops making progress, the system should not merely restart
the task with a larger model and discard everything learned.

It should escalate an evidence package:

- repository facts already inspected;
- the approved constraints;
- patches attempted;
- verification failures;
- plateau or timeout reason;
- commands required to establish success.

The more capable model should inherit the failed work as structured evidence,
not as an unbounded transcript. If it succeeds, the same deterministic gates
should judge the result.

This creates a useful local-first policy:

```text
Try locally
    ↓
Verify deterministically
    ↓
Pass → finish
    ↓
Plateau or timeout → escalate evidence
```

The economic question then becomes measurable: how much hosted-model cost can
local execution avoid without reducing the final verified success rate?

That is a better product question than whether one model can solve one issue
in one recording.

## What the failed repetitions proved

The 0/3 result was disappointing if the goal was to publish a successful local
agent benchmark.

It was valuable if the goal was to understand the system.

It showed that:

- one pass established capability but overstated reliability;
- the security-sensitive failure class remained unstable;
- longer time did not guarantee convergence;
- context size affected GPU residency and behavior;
- verbose stages were necessary to distinguish work from a hang;
- retry budgets needed a semantic plateau condition;
- negative artifacts contained more diagnostic value than a pass rate alone.

The first successful run is still real.

So are the next three failures.

A useful engineering account must be able to hold both facts at once.

The versioned
[fixture](https://github.com/jadercorrea/gptcode/tree/main/benchmarks/fixtures/go-safe-store),
[Devstral suite configuration](https://github.com/jadercorrea/gptcode/blob/main/benchmarks/suites/local-devstral-small-2-safe-store.json),
and
[result notes](https://github.com/jadercorrea/gptcode/blob/main/benchmarks/results/2026-07-29-local-filesystem.md)
are available in the GPTCode repository. The
[versioned experiment record](https://github.com/jadercorrea/ai-experiments/tree/2026.07.29.1/experiments/coding-agents/filesystem-containment/2026-07-29)
documents the protocol and limitations. Its
[release](https://github.com/jadercorrea/ai-experiments/releases/tag/2026.07.29.1)
publishes the historical run bundles, command logs, patches, verification
results, and repository snapshots under the SHA-256 checksum
`6ba0bc9bac3eac5ea9e740bb2e268923aa752f7e186902d5eb443d3a1df5054f`.

> One successful run demonstrates possibility.
>
> Engineering depends on repeatability.
