---
layout: post
title: "Capability Is Not Reliability"
date: 2026-07-29 09:00:00 -0300
author: Jader Correa
series: Evidence-Based AI Engineering
format: Technical brief
tags: [ai-agents, software-engineering, evaluation, reliability]
description: "A concise argument for evaluating coding agents through controlled repetition and executable evidence, not selected demonstrations."
image: /assets/agent-reliability-results.png
---

Yesterday I watched a local coding agent solve a security-sensitive task. It
changed one Go file, preserved the public API, passed the test suite under the
race detector, and passed static analysis. It used no hosted model and incurred
no API cost.

Then I ran the same configuration three more times.

Every run timed out.

That sequence captures a distinction we routinely blur when evaluating AI
systems:

> A successful run demonstrates capability. It does not establish reliability.

The first result answers whether the system *can* produce a valid solution. It
does not tell us how often the solution will appear, how sensitive the outcome
is to sampling, or whether the system is safe to use without supervision.

Those questions require controlled repetition and retained evidence.

<figure class="result-figure">
  <img
    src="{{ '/assets/agent-reliability-results.svg' | relative_url }}"
    alt="The capability campaign passed one of one runs. The subsequent repeatability campaign timed out in all three runs."
    width="1200"
    height="630"
  >
  <figcaption>
    One bounded capability success followed by a separate 0/3 repeatability
    campaign. The campaigns answer different questions.
  </figcaption>
</figure>

## Why demonstrations are persuasive

A good agent transcript resembles engineering work. The model inspects files,
forms a plan, edits code, runs tests, reacts to failures, and eventually
announces completion. When the final diff is correct, the trajectory feels
like proof.

But it remains one observation.

The visible run may also be the survivor of prompt changes, model selection,
context tuning, retries, and discarded attempts. The audience sees the coherent
result. The operator remembers the search process. If failed runs are not
preserved, neither can measure what the demonstration represents.

This is not unique to AI. Engineering disciplines distinguish a prototype that
works once from a process that performs consistently. Agent evaluation should
do the same.

## The experiment

The task was intentionally small but adversarial. A Go package wrote named
files beneath a configured root. The implementation had to preserve legitimate
nested paths while rejecting absolute paths, parent traversal, and escape
through a pre-existing symlink.

The executable contract also protected against a tempting shortcut. A filename
such as `reports/v1..v2.txt` had to remain valid. An earlier model appeared to
solve traversal by rejecting every string containing two adjacent dots. Human
review showed that it had prohibited a substring, not secured the filesystem.

After strengthening the fixture, I ran Devstral Small 2 locally through Ollama.
At a 16,384-token context, Ollama reported a 17 GB footprint and full GPU
residency on an Apple M1 Pro.

The capability check passed in 26 minutes and 8 seconds:

```text
go test -count=1 -race ./...    PASS
go vet ./...                    PASS
```

I then ran a separate three-repetition campaign. Every repetition started from
a fresh workspace, required a failing baseline, used the same model profile,
context, verification commands, retry limit, and 30-minute budget.

All three timed out.

Two attempts rejected legitimate nested paths. Another accepted an absolute
path and a symlink escape. The model understood much of the problem and
produced substantive changes. It did not converge within the declared budget.

The conclusion is neither “local models work” nor “local models are useless.”
It is narrower and more useful:

> This configuration demonstrated bounded capability. It did not demonstrate
> repeatable reliability.

## Evidence changes the engineering conversation

A pass rate alone is insufficient. To understand an agent run, I want the
initial repository, exact task and configuration, clean workspace, command
stream, produced patch, deterministic verification output, final snapshot,
duration, exit status, and the failures from unsuccessful repetitions.

Negative artifacts often contain more engineering information than the selected
success. They reveal whether the model misunderstood the contract, adopted an
unsafe approximation, entered a tool loop, regressed earlier progress, or
simply ran out of time near a valid solution.

The experiment also exposed failures in the harness itself. A provider timeout
was shorter than the suite budget. Long-running output was captured but hidden
until completion. Equivalent verification failures could consume the remaining
retry budget. A final successful verification could be reported as failed
because intermediate errors were treated as authoritative.

Fixing those defects did not make the model more capable. It made the system
more observable, more economical, and more honest when the model was not
capable enough.

## Tests define the boundary of the claim

Even the successful run should not be overstated. It passed the declared
traversal, absolute-path, and pre-existing-symlink cases. Human review found a
possible check-to-write race if a hostile concurrent process changed the
filesystem after validation.

The patch satisfied the fixture. It was not a universal race-free filesystem
primitive.

Executable verification is stronger than accepting the model's assertion, but
it proves only the properties encoded by the contract. Human review still has
to decide whether those properties are sufficient for the environment.

## Local inference needs an escalation policy

I expect local AI to follow part of the path that recording technology followed.
When I began recording music, serious work usually required access to a studio.
Today, much of that work can be done well at home. Professional studios remain
valuable for specialized rooms, equipment, expertise, and larger productions.
What changed is that centralized infrastructure is no longer mandatory for
every recording.

Frontier models, serverless inference, and managed MLOps will remain important.
But every repository question and bounded code change should not necessarily
require the largest hosted model.

The practical problem is identifying the boundary.

A local-first agent should try the lower-cost model, verify the result
deterministically, and finish when the contract passes. When progress plateaus
or the budget expires, it should escalate a structured evidence package to a
more capable model: repository facts, constraints, attempted patches,
verification failures, and the commands that define success.

That turns the economic question into something measurable: how much hosted
inference can local execution avoid without reducing the final verified success
rate?

## What to measure next

One pass does not justify a deployment policy. Neither does one failed batch.
The next useful evidence comes from predeclared campaigns across multiple
fixtures, model configurations, and failure classes.

The important unit is not the memorable transcript. It is the repeated,
restorable, independently inspectable run.

The complete engineering paper,
[One Successful Agent Run Proves Almost Nothing]({% post_url 2026-07-29-one-successful-agent-run-proves-almost-nothing %}),
documents the task, fixture corrections, failure modes, harness changes, local
inference argument, and human-review boundary. The
[2026.07.29.1 evidence release](https://github.com/jadercorrea/ai-experiments/releases/tag/2026.07.29.1)
contains the historical patches, command logs, verification results, and
repository snapshots.

One successful run demonstrates possibility.

Engineering depends on repeatability.
