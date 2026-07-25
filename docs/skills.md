---
layout: default
title: Skills
description: Reusable engineering guidance included with the open-source GPTCode CLI.
permalink: /skills/
---

# Skills

Skills are Markdown playbooks that add language conventions, testing practices, and review criteria to a GPTCode session. They are stored locally, can be inspected before use, and can be adapted to a project's needs.

## How skills work

```text
You run: gt do "add user authentication"
                     ↓
GPTCode detects the project language and framework
                     ↓
Relevant local skills are added to the agent context
                     ↓
The agent edits the project and runs its verification commands
```

Skills improve consistency, but they are instructions rather than guarantees. Generated code still requires tests and human review.

## Included skills

### Languages and frameworks

| Skill | Focus |
|:---|:---|
| Go | Error handling, interfaces, concurrency, and standard layouts |
| Elixir | Pattern matching, OTP, Phoenix contexts, and Ecto |
| Ruby | Readable object design and Ruby conventions |
| Rails | Controllers, services, Active Record, and RSpec |
| Python | PEP 8, type hints, pytest, and common Python patterns |
| TypeScript | Strict typing, generics, and safe asynchronous code |
| JavaScript | Modern modules, async flows, and DOM safety |
| Rust | Ownership, error handling, traits, and Cargo workflows |

### Engineering workflows

| Skill | Focus |
|:---|:---|
| TDD bug fix | Reproduce a defect with a failing test before changing production code |
| Code review | Review changes for correctness, security, and maintainability |
| Git commit | Prepare focused commits using Conventional Commits |
| Security | Apply practical OWASP-oriented checks |
| QA automation | Plan unit, integration, E2E, and accessibility coverage |

## Manage local skills

```bash
gt skills list
gt skills install ruby
gt skills install-all
gt skills show ruby
```

The project is open source, so the bundled skill definitions can be reviewed and improved directly in the [repository](https://github.com/jadercorrea/gptcode).
