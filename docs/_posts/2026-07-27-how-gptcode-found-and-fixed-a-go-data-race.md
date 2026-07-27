---
layout: post
title: "How GPTCode Found and Fixed a Real Go Data Race"
description: "A reproducible case study in repository-grounded research, adversarial review, API-preserving repair, and deterministic verification with Go's race detector."
date: 2026-07-27 10:00:00 -0300
author: Jader Correa
categories: [engineering, agents, go]
tags: [go, concurrency, ai-agents, verification, testing]
permalink: /blog/how-gptcode-found-and-fixed-a-go-data-race/
---

The interesting part of an AI coding agent is not whether it can produce a
plausible patch. It is whether the system can distinguish a plausible patch
from a correct one.

I used GPTCode against a deliberately small Go session store to test that
distinction. The store had a mutable map, an expiration rule, and a stable
public API. It also had a real concurrency defect.

The workflow did not succeed because the model was always right. It succeeded
because the model was allowed to be wrong without becoming the source of truth.

## The defect

The initial implementation stored sessions in a plain map:

```go
type Store struct {
    sessions map[string]Session
}

func (s *Store) Put(session Session) {
    s.sessions[session.Token] = session
}

func (s *Store) Active(token string, now time.Time) bool {
    session, ok := s.sessions[token]
    return ok && now.Before(session.ExpiresAt)
}
```

Each method looks reasonable in isolation. Together, they permit concurrent
reads and writes to the same map without synchronization.

The behavioral tests passed. That did not make the implementation safe.

```bash
go test ./...
```

Ordinary test success established that expected examples worked. It did not
establish the absence of races.

## The first answer was wrong

I started with a repository question:

```bash
gptcode research \
  "How does session expiration work, and is concurrent access actually safe?"
```

The first model response described concurrent access as safe. It saw
concurrency-oriented tests and inferred an implementation guarantee that the
code did not provide.

This is a common failure mode in AI-assisted engineering: documentation and
tests express intent, and a model quietly promotes that intent into a claim
about runtime behavior.

GPTCode now treats those sources differently:

- contracts and documentation describe expectations;
- implementation evidence describes the mechanism;
- executable verification determines whether the mechanism satisfies the
  expectation.

For a concurrency-safety claim, goroutines and a `sync.WaitGroup` in a test are
not synchronization for the production map. The implementation must contain a
lock, atomics, channels, or a documented confinement boundary.

When the research output contradicted that deterministic evidence, GPTCode
rejected the conclusion and requested a corrected answer. If the correction
still contradicts the inspected implementation, research fails instead of
publishing an unsupported safety claim.

## Review as an adversarial stage

Research maps the repository. Review challenges the conclusion:

```bash
gptcode review session/store.go \
  --focus "concurrency correctness and public API stability"
```

The focus matters. The task was not “redesign the store.” It was:

1. identify the evidenced defect;
2. preserve exported constructors and methods;
3. avoid inventing lifecycle requirements;
4. cite the supplied file and line evidence.

This separate stage confirmed the mismatch: the store exposed operations that
could execute concurrently, while the mutable map had no synchronization.

One model rarely excels equally at exploration, implementation, and criticism.
Explicit stages let each step use independent context, criteria, and even a
different provider.

## Turn the failure into an executable contract

A race detector only observes races exercised during a run, so the test must
actually create contention:

```go
func TestStoreSupportsConcurrentAccess(t *testing.T) {
    var store Store
    var workers sync.WaitGroup

    for worker := 0; worker < 32; worker++ {
        workers.Add(1)
        go func(worker int) {
            defer workers.Done()

            token := fmt.Sprintf("token-%d", worker)
            store.Put(Session{
                Token:     token,
                ExpiresAt: time.Now().Add(time.Hour),
            })
            _ = store.Active(token, time.Now())
        }(worker)
    }

    workers.Wait()
}
```

Against the vulnerable implementation:

```text
WARNING: DATA RACE
Read at ...
Previous write at ...
```

The important transition is from “the reviewer believes this is unsafe” to
“the repository contains a command that demonstrates the failure.”

## Repair the implementation, preserve the contract

The implementation task was deliberately bounded:

```bash
gptcode do \
  "Fix the session store concurrency bug without changing its public API. Verify with go test -race ./..."
```

The repair added private synchronization:

```go
type Store struct {
    mu       sync.RWMutex
    sessions map[string]Session
}

func (s *Store) Put(session Session) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.sessions == nil {
        s.sessions = make(map[string]Session)
    }
    s.sessions[session.Token] = session
}

func (s *Store) Active(token string, now time.Time) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()

    session, ok := s.sessions[token]
    return ok && now.Before(session.ExpiresAt)
}
```

The exported API did not change. The zero value remained useful. Reads can
proceed concurrently, while writes retain exclusive access.

The model proposed the change. The repository decided whether to accept it.

## Verification is the conclusion

The public fixture is intentionally small enough to audit. Its gate runs
behavior tests, concurrent access under the race detector, and statement
coverage:

```bash
make evidence
```

Current output:

```text
ok  github.com/jadercorrea/gptcode/examples/sessionstore
    coverage: 100.0% of statements
public examples verified: race detector passed; statement coverage 100.0%
```

CI fails if the race detector fails or if coverage for this fixture differs
from `100.0%`.

You can inspect the
[implementation and tests](https://github.com/jadercorrea/gptcode/tree/main/examples/sessionstore)
and the
[verification script](https://github.com/jadercorrea/gptcode/blob/main/scripts/verify-public-examples.sh).

## What this proves

This case study proves a narrow set of things:

- GPTCode can ground research in inspected repository contents;
- unsupported concurrency conclusions can be rejected before publication;
- review can enforce an explicit compatibility boundary;
- implementation can preserve the public API while changing private
  synchronization;
- the resulting fixture passes behavioral tests, Go's race detector, and its
  scoped coverage contract;
- another engineer can run the same deterministic verification.

That is enough to make the workflow falsifiable.

## What this does not prove

It does not prove that:

- every GPTCode result is correct;
- every possible schedule has been explored;
- the entire GPTCode repository has 100% coverage;
- a passing race-detector run proves the absence of every concurrency defect;
- model-backed steps are deterministic.

The 100% figure applies only to
[`examples/sessionstore`](https://github.com/jadercorrea/gptcode/tree/main/examples/sessionstore).
The legacy repository has uneven coverage, and the maintained workflows are
being improved while experimental surfaces are evaluated or retired.

Those limitations are part of the evidence, not footnotes to hide.

## The architecture behind the result

The workflow separates three forms of authority:

```text
Models        generate possibilities.
Repositories  define constraints.
Verification  establishes truth.
```

Research and review can still fail. Implementation can still produce a bad
patch. The system becomes more reliable when those failures are observable,
bounded, and checked by mechanisms outside the model.

That is the working thesis behind GPTCode:

> The workflow is the source of truth. Models are interchangeable execution
> engines.

Run the fixture with `make evidence`, inspect the
[full terminal demonstration](https://gptcode.dev/#workflow), or read
[the project quality contract](https://github.com/jadercorrea/gptcode/blob/main/QUALITY.md).
