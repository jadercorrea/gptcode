---
layout: post
title: "The Workflow Is the Source of Truth"
date: 2026-07-25
author: Jader Correa
tags: [ai-agents, software-engineering, verification, workflows, open-source]
description: "Why reliable coding agents need explicit stages, repository-native constraints, and executable verification—and what a failed GPTCode experiment taught me about the gap between producing an artifact and producing evidence."
---

Software teams are increasingly placing language models inside development
workflows. The models can search a repository, propose an architecture, edit
files, review a diff, and explain a failure. The capability is real.

So is the temptation to turn all of that capability into one large prompt:

> Understand this codebase, implement the feature, run the tests, fix any
> problems, and tell me when you are done.

That interaction is convenient. It is also a poor foundation for a reliable
engineering system.

A model can produce a convincing explanation without inspecting the right
files. It can produce a plan based on an invented architecture. It can report
that verification succeeded without establishing which commands ran, against
which revision, or with what result.

The problem is not that models are useless. The problem is that fluent output
can make several different responsibilities look like one.

My working principle for GPTCode is:

> **The workflow is the source of truth. Models are interchangeable execution
> engines.**

That principle can be reduced to three lines:

> **Models generate possibilities.**
>
> **Repositories define constraints.**
>
> **Verification establishes truth.**

This article explains why those responsibilities should remain separate, why
coding-agent workflows need explicit stages, and what happened when I tested
that thesis against GPTCode itself.

## A prompt is not a workflow

A prompt describes an intention. A workflow establishes a sequence of states,
inputs, outputs, permissions, and checks.

The distinction matters because software development is not a single cognitive
operation. Understanding a system is different from deciding how it should
change. Editing code is different from reviewing the edit. A plausible answer
is different from an executable result.

Collapsing those activities into one model call creates several problems:

1. **The evidence disappears.** Research, assumptions, decisions, edits, and
   checks become parts of one transcript instead of inspectable artifacts.
2. **Errors propagate invisibly.** Incorrect research becomes the foundation
   of an apparently coherent plan and implementation.
3. **Authority becomes ambiguous.** The same component that proposes a change
   can quietly redefine the criteria used to approve it.
4. **Recovery becomes expensive.** When the final result is wrong, the system
   cannot identify which stage introduced the failure.
5. **Models become architectural dependencies.** Changing a provider or model
   risks changing the entire behavior of the development process.

This is why GPTCode models development as:

```text
Research → Plan → Implement → Review → Verify
```

The stages are not ceremony. Each creates a boundary where the system can
inspect evidence, reject invalid state, choose a different model, or stop
before causing more damage.

## The five stages have different contracts

One model rarely excels at every task. More importantly, every task should not
have the same authority.

### 1. Research establishes observed context

Research should answer questions about the repository as it exists:

- Which modules participate in this behavior?
- Where are the public boundaries?
- Which tests encode the current contract?
- Which assumptions are confirmed by files, symbols, or commands?
- What remains unknown?

The output is not “analysis” in the abstract. It is a set of claims with
references to repository evidence.

A useful research artifact must make unsupported claims visible. If the agent
cannot locate the authentication boundary, it should say so. Guessing that a
project “probably uses Python” is not a partial success; it is a grounding
failure.

### 2. Planning turns context into a decision

A plan should not repeat the user request with more words. It should translate
observed context into:

- files and contracts expected to change;
- constraints that must remain true;
- migration and compatibility concerns;
- failure modes;
- verification commands;
- explicit unknowns requiring a decision.

Planning is where product intent and engineering constraints meet. The model
can propose options, but the workflow must preserve which facts came from the
repository and which choices came from a human.

### 3. Implementation operates within approved boundaries

Implementation converts an approved plan into a diff. Its authority should be
narrower than its capability.

An implementation agent may be technically capable of editing any file. That
does not mean the workflow should allow it to. The plan, repository policy, and
tool permissions should constrain the operation.

This separation also makes retries safer. A failed edit can be retried without
rerunning the entire reasoning process or silently changing the original plan.

### 4. Review challenges the first answer

Review should be independent enough to disagree with implementation.

Its purpose is not to restate the diff. It should test assumptions:

- Did the implementation preserve public contracts?
- Did it solve the requested problem rather than a nearby one?
- Are tests proving behavior or merely executing code?
- Did the change introduce security or operational risk?
- Does the result still match the approved plan?

Using a separate model can help, but model diversity alone is not independence.
The review stage also needs the original constraints and the actual diff.

### 5. Verification establishes executable truth

Verification belongs to deterministic tools whenever possible:

- tests;
- linters;
- type checkers;
- format checks;
- security scanners;
- repository-specific validation commands;
- release and production evidence.

A model may choose a relevant command or explain its output. It should not be
the authority that declares the command successful.

The exit code, output, revision, and environment are the evidence.

## Repository-native knowledge is part of the control plane

Generic model instructions are insufficient for a real codebase.

A repository contains local knowledge that changes what “correct” means:

- language and framework conventions;
- architectural boundaries;
- public API contracts;
- security rules;
- testing strategy;
- deployment constraints;
- review checklists;
- operational procedures.

GPTCode represents part of this knowledge as version-controlled
[skills]({{ '/skills' | relative_url }}). A Go skill can require explicit error
handling and small consumer-defined interfaces. An Elixir skill can describe
pattern matching, OTP behavior, and supervision concerns.

Because skills live close to the code, they can be reviewed, versioned, and
changed through the same process as other engineering decisions.

They are still instructions, not proof. A skill may tell an agent to run
`mix test`; only the test process can establish whether the suite passed.

## The first experiment failed

While preparing the terminal demonstration for the
[GPTCode homepage](https://gptcode.dev), I created a deliberately small Go
fixture.

The fixture contained:

- a Go module;
- an in-memory session store;
- explicit expiration behavior;
- behavior and concurrency tests;
- a short README describing the package.

The deterministic language detector inspected the repository and reported:

```text
Go               86.5%
Markdown         11.1%
Go Module         2.4%
```

Then I ran:

```bash
gt research "Document the session lifecycle, expiration boundary, and verification evidence"
```

The command created a research document, but the content was not grounded in
the fixture. It said the repository was “likely Python” and recommended
inspecting conventional `src/`, `tests/`, and `docs/` directories that did not
exist.

The artifact was generated successfully.

The research was wrong.

That distinction is the entire point of this article. A file saved to disk is
not evidence that research occurred. A green command status is not evidence
that the result is semantically correct.

I also tried the review stage. It remained pending for several minutes without
producing a useful result and had to be interrupted. A model-backed command
without a meaningful timeout or progress contract is not a reliable workflow
stage.

I did not publish that run as a success story. The software had not earned the
demonstration yet.

## Failure is useful when the boundary is visible

The failed experiment produced better architectural requirements than a
polished mock would have.

### Research needs a grounding contract

A research stage should not succeed unless its claims reference observable
repository facts. At minimum, the result should include:

- files inspected;
- symbols or sections used as evidence;
- commands executed;
- unresolved questions;
- confidence or grounding status.

If the agent references nonexistent paths or contradicts deterministic language
detection, the workflow should reject the artifact.

### Model-backed stages must fail fast

Every stage needs:

- a deadline;
- visible progress;
- cancellation;
- a typed failure result;
- enough context to retry safely.

“Still thinking” is not a state an engineering workflow can accept
indefinitely. A review that exceeds its deadline should fail fast and preserve
the inputs needed for another model or a human reviewer.

### Verification must be bound to a revision

“Tests passed” is incomplete evidence.

A verification record should identify:

- the command;
- the working directory;
- the source revision or diff;
- the exit code;
- relevant output;
- the environment;
- the timestamp.

Without that binding, a passing test may describe a different state from the
one being approved.

### The workflow must be able to disagree with the model

This is the most important requirement.

If deterministic repository evidence says the project is Go and model research
says it is likely Python, the workflow should not reconcile the conflict by
writing a confident summary. It should stop.

Reliable systems need explicit disagreement states.

## Turning the failure into an executable contract

The failed run became a set of implementation requirements:

- research must receive repository evidence and reject contradictions;
- review must inspect the requested file and preserve the original focus;
- implementation must retain the original task and repository constraints;
- exported Go APIs must be compared before and after a change;
- requested verification commands must be explicitly allowed and executed by
  the system rather than merely suggested by a model.

After implementing those boundaries, I repeated the experiment with an
intentionally unsafe in-memory session store.

Research located the unsynchronized map and cited the implementation and
concurrency test. Review independently identified the same defect. The
implementation stage added a private `sync.RWMutex`, preserving every exported
type and method. The system then executed the requested verification:

```text
Verifying: go test -race ./...
[OK] Verification passed

diff --git a/session/store.go b/session/store.go
+       mu       sync.RWMutex
+       s.mu.Lock()
+       defer s.mu.Unlock()
+       s.mu.RLock()
+       defer s.mu.RUnlock()

ok      example.com/sessionstore/session
✓ Diagnosed · reviewed · repaired · verified
```

The terminal recording on the homepage is that real run. The public
[session-store fixture](https://github.com/jadercorrea/gptcode/tree/main/examples/sessionstore)
keeps the example auditable and adds a stronger repository contract:

```bash
scripts/verify-public-examples.sh
```

That command runs the behavior suite with Go's race detector and rejects the
example unless statement coverage remains exactly 100%.

## What “source of truth” means

Calling the workflow the source of truth does not mean the workflow is always
correct. It means correctness is established through explicit transitions and
evidence rather than inferred from the confidence of the last response.

The workflow records:

```text
Observed repository state
        ↓
Research claims with evidence
        ↓
Approved plan and constraints
        ↓
Implementation diff
        ↓
Independent review findings
        ↓
Verification bound to the resulting revision
```

Models can participate throughout this pipeline. They can be upgraded,
replaced, routed by task, or run locally. The engineering contract remains
stable because the workflow—not the provider—defines what each stage must
produce.

This is also why provider independence matters. OpenAI, Gemini, Groq,
OpenRouter, Ollama, or a future model should be an execution engine behind a
stage contract, not the architecture of the system.

## Reliable AI systems are built on explicit constraints

Optimistic prompts assume that capability will produce correctness:

> Be careful. Inspect the repository. Follow best practices. Run all tests.

Explicit systems make those expectations enforceable:

- restrict which files may change;
- require evidence for research claims;
- preserve approved constraints across stages;
- capture the exact verification commands and results;
- time out model-backed operations;
- expose disagreement instead of smoothing it over.

Prompts remain useful. They are part of the implementation of a stage. They are
not the control plane.

## GPTCode is an executable research project

GPTCode is not proof that this problem has been solved. It is the tool I use to
make the hypothesis testable.

Its current strengths and remaining limitations are both valuable:

- multi-provider workflows test whether models can remain replaceable;
- repository-native skills test whether engineering knowledge can live with
  the code;
- explicit stages test whether agent work can remain inspectable;
- deterministic commands test where model authority should end;
- the public repair fixture makes the core claim independently reproducible;
- low coverage in legacy packages remains visible work rather than a hidden
  marketing claim.

That is what I mean by an idea in executable form.

The code, tests, skills, and current limitations are available in the
[GPTCode repository](https://github.com/jadercorrea/gptcode). The project
homepage at [gptcode.dev](https://gptcode.dev) presents the architecture and a
real terminal recording.

The next milestone is not a more impressive demo. It is applying the same
quality bar—explicit contracts, race-safe code, executable checks, and useful
coverage—to more of the legacy codebase.

---

*Jader Correa is a principal engineer and founder working on AI agents,
developer tools, and distributed systems.*
