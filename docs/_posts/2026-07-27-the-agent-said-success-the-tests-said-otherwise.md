---
layout: post
title: "The Agent Said Success. The Tests Said Otherwise."
description: "A real GPTCode security fix showing why repository evidence and executable verification must outrank confident agent output."
date: 2026-07-27 14:30:00 -0300
author: Jader Correa
categories: [engineering, agents, security]
tags: [ai-agents, security, go, testing, verification]
permalink: /blog/the-agent-said-success-the-tests-said-otherwise/
---

An agent declaring success is not evidence that the work is complete.

I used GPTCode to investigate and fix a real repository-boundary vulnerability
in GPTCode itself. The workflow produced useful reasoning, an incomplete patch,
several confident mistakes, and eventually a verified security fix.

That sequence is more representative of production AI engineering than a
perfect terminal demo.

## The vulnerability

GPTCode exposes filesystem tools to coding agents. The original `read_file`
implementation accepted a model-provided path and joined it to the repository
working directory:

```go
fullPath := filepath.Join(workdir, path)
content, err := os.ReadFile(fullPath)
```

There was no check that the result remained inside `workdir`.

A path such as `../outside/sentinel.txt` could escape the repository. A symlink
inside the repository could point to the same outside file. The read result
would then become part of the agent conversation and could be sent to the
configured model provider.

The same missing invariant affected listing, writing and patching.

I documented the defect, a safe reproduction and explicit acceptance criteria
in [issue #17](https://github.com/jadercorrea/gptcode/issues/17) before
implementing the fix.

## Research produced the wrong answer

I first asked GPTCode:

```text
Can model-invoked file tools read files outside the repository through path
traversal or symlinks?
```

The research stage found this statement in `QUALITY.md`:

```text
Research evidence stays inside repository boundaries.
```

It then inferred that protections were in place.

They were not.

The research artifact was grounded in a real repository document, but it did
not inspect `internal/tools/tools.go`, where the affected implementation lived.
It confused a stated quality contract with proof that every filesystem tool
satisfied that contract.

The artifact was useful precisely because it was inspectable. Its citations
made the missing evidence visible.

## Planning invented repository details

The planning stage correctly proposed a shared path resolver and a TDD
strategy. It also claimed that the implementation lived in:

```text
internal/tool/file.go
```

The real package was `internal/tools`, and no such file existed. The plan also
required `make lint`, a target the repository does not define.

The architecture was plausible. The repository facts were wrong.

An implementation plan should therefore be treated as a reviewable hypothesis,
not an execution contract accepted on generation.

## Implementation stopped after creating tests

I then ran the `do` workflow with exact constraints:

```text
First add failing tests using temporary sentinel files.
Then implement one shared repository-boundary resolver.
Apply it to read_file, list_files, write_file and apply_patch.
```

GPTCode created tests and reported:

```text
[OK] Task complete!
Changes applied; validation delegated to Maestro
STATUS: SUCCESS
```

Production code had not changed.

The generated tests also called nonexistent exported functions such as
`ReadFile` and `WriteFile`, so the package did not compile.

The status was optimistic model output. The repository state was evidence.

## Turning the attempt into a real failing test

I corrected the test harness without weakening its assertions. It created two
temporary sibling directories:

```text
temporary root
├── repository
└── outside
    └── sentinel.txt
```

No real user file or secret was read.

The test exercised five independent escapes:

- `read_file` with `../outside/sentinel.txt`;
- `read_file` through an in-repository symlink;
- `list_files` with `../outside`;
- `write_file` through path traversal;
- `apply_patch` through a symlink.

The red test proved the defect:

```text
read_file allowed path traversal
read_file allowed symlink escape
list_files allowed path traversal
write_file modified the outside sentinel: "overwrite"
apply_patch modified the outside sentinel: "hacked"
```

This was the first trustworthy answer in the workflow.

## The fix: one boundary, four tools

The implementation introduced one internal resolver:

```go
func resolveRepositoryPath(
    workdir string,
    requestedPath string,
    allowMissing bool,
) (string, error)
```

It enforces four rules:

1. model-provided paths must be relative;
2. lexical resolution must remain below the repository root;
3. existing symlinks must resolve below the real repository root;
4. writes to missing paths must have an existing ancestor inside that root.

Containment uses `filepath.Rel`, not string-prefix comparison:

```go
relative, err := filepath.Rel(root, candidate)
return err == nil &&
    relative != ".." &&
    !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
```

That distinction prevents prefix confusion between paths such as `/tmp/repo`
and `/tmp/repo-other`.

The resolver now guards `read_file`, `list_files`, `write_file` and
`apply_patch` without changing their model-facing schemas.

## Review failed differently

I asked GPTCode to review the entire `internal/tools` directory. The review
agent spent all five iterations invoking tools and returned an empty report.

I repeated the review against the exact resolver file. Direct evidence mode
produced a structured review and questioned the behavior for missing write
targets.

That behavior was intentional: a write resolver must return a safe destination
that does not exist yet. The important condition is that its deepest existing
ancestor resolves inside the repository.

The second review was useful, but it still required engineering judgment.

## Verification established the result

The focused package verification passed with the race detector:

```text
$ go test -race -cover ./internal/tools
ok github.com/jadercorrea/gptcode/internal/tools
coverage: 34.8% of statements
```

The repository-level gate also passed:

```text
$ make verify
```

That command verifies formatting, static analysis, the CLI build, the complete
short test suite, the public project contract and the race-tested example used
by the documentation.

The security tests additionally assert that rejected write and patch attempts
leave the outside sentinel unchanged.

## What this proves—and what it does not

The fix prevents model-selected paths from escaping the repository through
absolute paths, traversal, prefix confusion or existing symlinks. It preserves
normal in-repository reads and writes.

It is not a filesystem sandbox against a hostile concurrent process swapping
symlinks between validation and I/O. Closing that time-of-check/time-of-use
class completely requires operating-system-level primitives and a different
threat model.

The claim is deliberately narrower than “the agent is sandboxed.”

## The workflow is valuable because models can be wrong

Every model-driven stage contributed something:

- research exposed which evidence it had not inspected;
- planning proposed the right architectural shape but invented paths;
- implementation produced a useful test direction but stopped early;
- review challenged an intentional edge case;
- deterministic verification established the actual result.

The lesson is not that agents are unreliable and therefore useless.

The lesson is that useful agents must be placed inside a system where their
mistakes are observable and cannot declare themselves correct.

Models generate possibilities.

Repositories define constraints.

Verification establishes truth.
