# GPTCode

> **AI coding agents should produce evidence, not just answers.**

GPTCode is an open-source coding CLI for investigating, planning, implementing,
reviewing, and verifying software with multiple AI models. It keeps repository
knowledge, engineering constraints, and executable verification close to the
code.

[![CI Build & Test](https://github.com/jadercorrea/gptcode/actions/workflows/ci.yml/badge.svg)](https://github.com/jadercorrea/gptcode/actions/workflows/ci.yml)
[![Release](https://github.com/jadercorrea/gptcode/actions/workflows/cd.yml/badge.svg)](https://github.com/jadercorrea/gptcode/actions/workflows/cd.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[Website](https://gptcode.dev) ·
[Architecture](https://gptcode.dev/#architecture) ·
[Documentation](https://gptcode.dev/guides/getting-started) ·
[Engineering thesis](https://gptcode.dev/blog/the-workflow-is-the-source-of-truth/) ·
[Evaluation essay](https://gptcode.dev/blog/2026-07-29-one-successful-agent-run-proves-almost-nothing)

<p align="center">
  <a href="https://gptcode.dev">
    <img
      src="https://gptcode.dev/assets/gptcode-workflow.gif"
      alt="GPTCode investigates, reviews, repairs, and verifies a real Go concurrency defect"
      width="960"
    />
  </a>
</p>

## The thesis

Language models generate possibilities. Repositories define constraints.
Verification establishes truth.

GPTCode separates those responsibilities into explicit, inspectable stages:

```text
Repository knowledge
  Skills · contracts · tests · documentation
                    │
                    ▼
Research → Plan → Implement → Review → Verify
                    │
                    ▼
OpenAI · Gemini · Groq · OpenRouter · Ollama
```

The workflow is the source of truth. Models are interchangeable execution
engines.

## Install

GPTCode requires Go 1.24 or newer:

```bash
go install github.com/jadercorrea/gptcode/cmd/gptcode@latest
gptcode setup
```

Connect a provider:

```bash
gptcode key openrouter
```

Provider credentials remain local to your GPTCode configuration. Review the
commands and provider policies before using the CLI with confidential code.

## A real workflow

```bash
gptcode research \
  "How does session expiration work, and is concurrent access safe?"

gptcode review session/store.go \
  --focus "concurrency correctness and public API stability"

gptcode do \
  "Fix the session store concurrency bug without changing its public API. Verify with go test -race ./..."
```

The published demonstration uses a real Go fixture and ends with executable
evidence rather than a model assertion:

```text
Research   → identifies the shared-state boundary
Review     → confirms unsynchronized map access
Implement  → preserves the public API and adds synchronization
Verify     → go test -race ./...
```

See the [auditable fixture](examples/sessionstore) and the
[recording source](docs/assets/record-gptcode-workflow.zsh).

## Verify the evidence

Run the same deterministic checks used in CI:

```bash
make evidence
```

This executes the public fixture with Go's race detector and fails unless it
retains 100% statement coverage:

```text
ok  gptcode/examples/sessionstore  coverage: 100.0% of statements
public examples verified: race detector passed; statement coverage 100.0%
```

To run the complete repository quality gate:

```bash
make verify
```

That gate builds the CLI, runs `go vet`, executes the short test suite, checks
the public project contract, and runs the race/coverage evidence.

The complete claim-to-evidence matrix is documented in
[QUALITY.md](QUALITY.md).

## Repository-native skills

Skills are version-controlled engineering instructions loaded from the
repository. They can define language conventions, architectural boundaries,
testing practices, and review criteria:

```bash
gptcode skills list
gptcode skills show go
gptcode skills install ruby
```

## Core commands

| Command | Purpose |
| --- | --- |
| `gptcode research "question"` | Investigate code and produce repository-grounded findings |
| `gptcode plan "task"` | Turn ambiguity into an inspectable implementation plan |
| `gptcode do "task"` | Implement a bounded change with tool-backed verification |
| `gptcode review [path]` | Review correctness, security, and contract stability |
| `gptcode chat` | Explore a repository interactively |
| `gptcode skills list` | Inspect repository-native engineering guidance |

Use `gptcode --help` for the complete command surface.

## Design principles

- **Repository-centered:** project constraints outrank temporary prompt context.
- **Explicit stages:** research, planning, implementation, review, and
  verification remain inspectable.
- **Provider-independent:** route work according to capability, latency, cost,
  and privacy requirements.
- **Executable evidence:** tests, linters, race detection, and project commands
  decide whether a change is acceptable.
- **Honest boundaries:** model output is a proposal, never proof of correctness.

## Limitations

GPTCode is an independent research-driven project, not a hosted service or a
guarantee that model-generated changes are correct.

- Model-backed workflows require credentials for a supported provider.
- Results vary by model and repository context.
- The 100% coverage claim applies to the published `sessionstore` fixture, not
  to the entire legacy codebase.
- Some experimental command surfaces predate the current repository-centered
  architecture and are being evaluated or retired.
- Always review changes and run the repository's own verification commands.

## Development

```bash
git clone https://github.com/jadercorrea/gptcode.git
cd gptcode
make verify
```

Releases are intentional: pushing a version tag runs the full verification gate
before GoReleaser publishes checksums and platform archives to GitHub.

Read [CONTRIBUTING.md](CONTRIBUTING.md), [SUPPORT.md](SUPPORT.md), and
[SECURITY.md](SECURITY.md) before opening a contribution or reporting a
vulnerability.

## Project status

GPTCode is actively maintained as an open-source engineering laboratory for
reliable AI-assisted software development. The roadmap is public in
[_roadmap.md](_roadmap.md).

Created by [Jader Correa](https://jader-correa.com), a principal engineer
building AI agents, developer tools, and distributed systems.

## License

[MIT](LICENSE)
