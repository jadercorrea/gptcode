---
layout: post
title: "Experimental TDD for Agent Infrastructure"
date: 2026-09-14
author: Jader Correa
series: Evidence-Based AI Engineering
format: Experiment report
tags: [coding-agents, intermediate-representation, experimentation, verification, agent-infrastructure]
description: "I set out to test a semantic interface for coding agents. The failures taught me how much of the experiment itself needed testing first."
---

<aside class="paper-abstract" aria-label="Executive summary">
  <strong>The infrastructure has to earn the right to measure the agent.</strong>
  A compact representation reduced output tokens without necessarily reducing
  total interaction cost. Valid schemas did not guarantee valid model output.
  Successful reference replays missed a legal inspection path. This report
  follows those failures through a small semantic-IR experiment. It offers
  reproducible engineering evidence, not a claim that semantic interfaces make
  coding agents better.
</aside>

I have a habit of looking under the hood when something appears to work. As a
self-taught engineer, experimentation and observation have always been how I
make sense of things. AI agents have given that habit plenty to do.

In [The Transcript Is Not the State]({{ '/blog/2026-08-25-the-transcript-is-not-the-state' | relative_url }}),
I asked whether coding agents should operate on explicit semantic state instead
of repeatedly reconstructing it from transcripts and source files.

Then I tried to build the smallest useful experiment around that idea.

The attractive story was simple: give an agent a typed representation, a compact
instruction language, and deterministic validation. Perhaps it would spend less
effort reconstructing structure and more effort solving the task.

The actual story involved expensive JSON, misleadingly small outputs, schemas
the provider rejected, schemas it accepted without enforcing, and an inspection
label that broke an otherwise carefully checked runtime.

At some point, I noticed what we were doing: TDD on the experiment itself.

Not “keep changing the benchmark until my idea wins.” Almost the opposite:
identify the next assumption that could make a result uninterpretable, write an
executable check, and let it fail before spending more on model calls.

## First, the token bill disagreed

The first slice was deliberately narrow: normalize a user identifier, reject an
empty one, query a closed capability, and return an explicit result. The
semantic tree had stable identities, checked types and effects, an interpreter,
and deterministic TypeScript generation. No custom tokenizer or new model.

It worked. It was also expensive.

In the initial valid multi-turn comparison, the JSON IR arm consumed **4.25
times** the total tokens of direct TypeScript. That observation was limited to
one construction task, but it was enough to reject the comfortable assumption
that structure would automatically be cheaper.
([Recorded observation](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/TOKEN_CONSUMPTION_OBSERVATION.md))

A separate single-shot comparison isolated the representation more tightly:
one provider call per arm, the same task, no correction loop. Compact tuples
decoded into the same canonical IR and went through the same validator.

| Representation | Input tokens | Output tokens | Total tokens |
| --- | ---: | ---: | ---: |
| TypeScript | 1,751 | 258 | 2,009 |
| Canonical JSON IR | 3,542 | 945 | 4,487 |
| Compact IR | 1,906 | 116 | 2,022 |

All three passed the public and hidden behavior checks. Compact IR cut output
tokens by **55.04%**, yet used **0.65% more total tokens** than source. Teaching
the model the compact contract almost exactly consumed the output saving.
Under the frozen prices, its estimated cost was lower because output tokens
were more expensive than input tokens. These are different measurements, not
contradictory results.
([Single-shot report and raw evidence](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/SINGLE_SHOT_REPRESENTATION_OBSERVATION.md))

A fixed follow-up grid of 1, 2, 4, 8, and 16 repeated function bodies crossed
total-token break-even at eight bodies and stayed below source at sixteen.
That demonstrated amortization in this synthetic workload. It did **not**
establish an eight-function rule for real repositories. The source model could
also produce more concise code, and the observed advantage was smaller at
sixteen than at eight.
([Break-even curve](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/BREAK_EVEN_OBSERVATION.md))

JSON had not invalidated the semantic model. It had exposed the cost of one
serialization and its surrounding contract. A smaller answer was not the same
thing as a cheaper interaction.

## Then the interaction became the problem

Moving from whole programs to patches introduced another set of costs:
inspection, available symbols, capabilities, preconditions, tool definitions,
retained context, and repair turns.

We worked through those layers one at a time. The recursive patch language
became a flatter vocabulary called Motion. The callable surface became a small
Session ISA—an instruction set for interacting with the runtime, not a CPU
instruction set. Accumulated history became explicit current state, followed by
a bounded working set of semantic observations.

Those changes produced real local engineering improvements. They did not
reliably produce better task outcomes.

Calibration 004 is a useful example. Across its six tasks, source passed two
hidden evaluations and semantic passed one. On the five supported pairs, the
semantic arm emitted **18.0% fewer output tokens**, but consumed **78.6% more
total tokens** and **55.5% more estimated cost**.
([Calibration 004](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/CALIBRATION_004_OBSERVATION.md))

These small, development-exposed calibrations are descriptive. They are not
independent confirmatory trials, and I cannot pool successive redesigned
interfaces into a general performance estimate.

Still, they changed what I looked at. Counting the patch alone was plainly
insufficient. The experiment had to charge for everything needed to understand,
produce, validate, and repair that patch.

## A schema can pass three different tests

One of the most useful failures happened before the model could even act.

Our first provider probe failed because the tool schema lacked an explicit
root `type: object`. The second passed that check but failed because the
endpoint rejected a top-level union. A third version nested the instruction
under a wrapper. Both probe requests then reached generation, and both sampled
calls validated.
([Probe 003, including the preceding failures](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/PROVIDER_SCHEMA_CAPABILITY_PROBE_003_OBSERVATION.md))

It would have been easy to call that “grammar enforcement.” It wasn't.

There are at least three separate questions:

1. Does the local validator recognize the intended language?
2. Does the provider accept the schema in a request?
3. Does generation actually stay inside that language?

The first live confirmatory canary later supplied a direct counterexample to
conflating them: the model emitted a string where the admitted schema required
an array. Local validation rejected it. The safety boundary worked; generation
had not been constrained as we might have hoped.
([Canary 001](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/REPRESENTATIONAL_CONFIRMATORY_CANARY_001.md))

Rejecting an invalid action protects the workspace. Helping the agent reach a
valid, useful action is a separate problem.

## The reference path was not the reachable state

The next study narrowed the question. Instead of changing source code into a
semantic interface and attributing every difference to “representation,” it
varied two properties of semantic inspection results:

- meaningful labels versus opaque labels;
- nested records versus a flat table with references.

All four realizations decoded into the same canonical observation. This 2×2
design deliberately has no source-code arm: it asks about wording and packaging
*within* the semantic interface. It is not a test of whether IR beats Python or
TypeScript.
([Factorial design](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/REPRESENTATIONAL_DEPENDENCE_DESIGN_V0.md))

The frozen cohort contains 240 tasks and 960 scheduled condition cells. Local
reference replays passed. Then canary 002 took a legal inspection path those
references had not covered.

The observation contained `arguments[0]`.

That label was outside the opaque codec's closed vocabulary. The run stopped
because the infrastructure could not express something the agent was legally
allowed to inspect. Counting that as a model failure would have contaminated
the comparison.

We kept the invalid run and versioned the repair. The successor check covered
all 3,552 reachable nodes in the frozen cohort, with encode/decode round trips
across all 960 cells. This is cohort-bounded lexical totality: an exhaustive
local property of the specified inspection surface, not a universal proof about
future programs or every possible agent session.
([Failure and totality check](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/REPRESENTATIONAL_LEXICALIZATION_TOTALITY_V0.md))

That distinction is the experiment in miniature. “The reference solution
works” is weaker than “the interface covers the legal observations the agent
can request.”

## Fixing the observation language did not finish the task

Canary 003 used the next immutable schedule cell under the versioned protocol.
It completed twelve provider turns without the lexicalization failure. It
still exhausted its budget without a terminal submission.

Three attempted submissions reached progressively deeper validation failures:
the outer instruction's field count, the patch identifier's grammar, then a
root reference's grammar. No mutation was applied and no hidden evaluation ran.

The operational gate passed. The task did not.
([Canary 003, including requests, accounting, and claim boundaries](https://github.com/jadercorrea/ai-experiments/blob/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26/REPRESENTATIONAL_CONFIRMATORY_CANARY_003.md))

Mulling over that trajectory, I started asking a slightly different question:
what good is a complete observation language if the agent must discover each
construction rule through another rejected submission?

The next hypothesis is about the other direction of the interface: legal
actions must be expressible, and invalid actions should return bounded,
machine-addressable diagnostics. An error code, structural path, expected
production, observed value, and recoverability flag are testable objects. “That
patch is invalid” leaves much more reconstruction to the next model turn.

This is a proposed intervention, not evidence that structured diagnostics would
have rescued canary 003. Expressibility and a clear error do not guarantee that
a model can find the repair, much less understand the requirement.

## What I mean by experimental TDD

Ordinary TDD asks for a failing test before an implementation change. Here, the
red test often concerns the *right to interpret the next measurement*.

Can the representation express the task? Does the compact transport preserve
meaning? Are both arms paying comparable interface costs? Can the provider
accept the grammar? Does a legal inspection crash the codec? Can historical
evidence still be loaded without silently rebuilding it under today's rules?

The loop I want to preserve is:

1. State a narrow assumption and its failure criterion.
2. Freeze the relevant interfaces, fixtures, accounting, and authority.
3. Run a local check where possible; use a bounded provider probe where needed.
4. Preserve failures and classify infrastructure defects separately from task outcomes.
5. Version the intervention before collecting new observations.

The analogy has limits. A unit test can establish a deterministic invariant;
it cannot establish a population-level behavioral effect. Replaying recorded
actions can test a new runtime, but it does not tell us which actions the model
would have chosen under that runtime. And repeated calibration can overfit the
interface to the development tasks.

That is why the frozen schedule, fresh subjects, unchanged historical evidence,
and explicit launch gates matter. A failed canary is not permission to retry
until the demonstration looks convincing.

## What is ready to publish—and what isn't

As of this report, only schedule sequences 1–3 have external observations.
Canary 002 is infrastructure-invalid; canaries 001 and 003 are operationally
admitted task failures, with a protocol change between them. The remaining
957 cells are blocked. This is not a three-sample efficacy estimate, and the
960-cell cohort is not a completed benchmark.

I have not demonstrated a superior language for coding agents. I have
demonstrated some costs that a plausible design concealed, and built checks
that make the next comparison harder to fool ourselves with.

That seems worth sharing while the work is still untidy. If someone else is
building an agent protocol, a semantic editing interface, or a benchmark, they
can reuse the distinction between schema admission and enforcement, test paths
beyond their reference solution, and count the full interaction before claiming
a token saving.

The [published experiment trail](https://github.com/jadercorrea/ai-experiments/tree/44ecaea300d7e0ba003f59194b8097857b5800fe/experiments/coding-agents/semantic-ir/2026-08-26)
contains the protocols, failed observations, versioned constructions, and
evidence links. References in this article are pinned to a repository revision
so later work does not quietly change what they support.

I still think explicit semantic state is worth investigating. The experiment
has made me less confident in the shortcuts and more interested in the actual
boundary between the model and its tools.

For now, the useful result is knowing which test needs to go red next.

---

**A note on authorship:** The experiments and argument are mine. AI helped me
research, challenge, implement, and edit the work—and made my typing and English
considerably less painful to read. I reviewed the final text and stand behind it.
