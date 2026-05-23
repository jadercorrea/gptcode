---
layout: default
title: Agent Skills — Built-in Premium Capabilities
description: Built-in capabilities of the pre-compiled Live CLI Agent. Learn how language-specific and general skills are centrally managed for secure, high-performance, and idiomatic local code execution.
permalink: /skills/
---

# Agent Skills

**Skills are the built-in expertise engines** that power the `gt` pre-compiled CLI agent. Every `gt` session automatically leverages these secure capabilities to generate production-quality, enterprise-grade code that adheres to industry guidelines and strict framework patterns.

> 💡 **Unified Subscription Advantage**: Under the **gptcode live** subscription, your compiled CLI agent automatically retrieves, verifies, and updates these skills from your central organization control plane. This ensures every developer on your team compiles, edits, and reviews code with identical, highly optimized standards.

---

## Why Built-in Skills Matter

Without enterprise skills, raw AI models produce generic, unoptimized code that lacks security context or language idioms. With `gt`'s built-in skills, your pre-compiled agent natively produces **production-ready code**:

*   **Idiomatic Patterns**: Follows precise language conventions (e.g., explicit error handling in Go, pattern matching and OTP architectures in Elixir, type-safety in TypeScript, and lifetime constraints in Rust).
*   **Strict Security Norms**: Natively incorporates input sanitization, OWASP guidelines, and vulnerability mitigation (XSS, CSRF, and SQL Injection prevention) as part of its generation path.
*   **Consistent Testing & Formatting**: Automatically implements test-driven designs (TDD) using standard frameworks and applies exact project styling patterns.

---

## How Skills Work Natively

```
┌──────────────────────────────────────────────────────────┐
│  You run: gt do "add user authentication"                │
│                        ↓                                 │
│  gt detects: Ruby on Rails project (Gemfile, config/)   │
│                        ↓                                 │
│  gt fetches: Verified Rails & Ruby skills from Live      │
│                        ↓                                 │
│  gt executes: Generates idiomatic service objects,       │
│               RSpec tests, and runs local verification   │
└──────────────────────────────────────────────────────────┘
```

Because skills are managed centrally via the **gptcode live** control plane, they run locally within the pre-compiled binary sandbox. Your source code never leaves your local machine, and your proprietary prompt blueprints remain secure.

---

## Available Built-in Skills

### 1. Language-Specific Engineering

| Skill | Primary Focus | Best Practices Enforced |
|:---|:---|:---|
| **Go** | Performance & Concurrency | Explicit error handling, interfaces, goroutines, and standard project layouts. |
| **Elixir** | Fault Tolerance & Scaling | Pattern matching, OTP supervisors, Phoenix contexts, and Ecto query safety. |
| **Ruby** | Readable MVC | Clean method designs, modular patterns, Active Record optimization, and dry code. |
| **Rails** | Rapid Web Dev | Service objects, thin controllers, secure migrations, and clean RSpec test structures. |
| **Python** | Data & Type Safety | PEP 8 styling, strict type hinting, robust pytest fixtures, and list comprehensions. |
| **TypeScript** | Strict Typing & Async | Generics, interface contracts, safe async/await patterns, and React-specific idioms. |
| **JavaScript** | Modern ES6+ | Clean async flows, modular designs, performant array manipulation, and DOM-safe operations. |
| **Rust** | Safety & Performance | Correct ownership management, error handling, trait systems, and cargo layouts. |

### 2. General Development Standards

| Skill | Focus | Operational Mechanics |
|:---|:---|:---|
| **TDD Bug Fix** | Test-Driven Design | Automatically generates failing tests before writing the implementation to guarantee fix efficacy. |
| **Code Review** | High Quality | Automatically reviews code changes locally against performance, safety, and naming rubrics. |
| **Git Commit** | Clean History | Standardizes commit histories using strict Conventional Commits syntax tied to Live issues. |

### 3. Product & Design Delivery

| Skill | Focus | Implementation Patterns |
|:---|:---|:---|
| **Design System** | UI Consistency | Atomic Design principles, token-based variables, Storybook mappings, and WCAG accessibility. |
| **Product Metrics** | Analytics Integrity | Proper instrumentation of funnels, UTM trackers, telemetry pixels, and data events. |
| **Production Ready** | Robustness | Heavy error handling, feature flagging, health checking, and graceful degradation. |
| **QA Automation** | Reliability | Automated E2E testing (Playwright/Cypress), visual regression systems, and performance budgets. |

### 4. Enterprise Ops & Security

| Skill | Focus | Deployment Patterns |
|:---|:---|:---|
| **Security** | Compliance | Enforces OWASP Top 10 defenses, sanitizes inputs, and handles credentials safely. |
| **DevOps** | Infrastructure | Compiles Docker containers, drafts Kubernetes configs, and constructs Terraform modules. |
| **SysOps** | Shell & Linux | Safe shell scripts, unit test scripts, systemd configurations, and system administration. |
| **SecOps** | Threat Management | Integrates vulnerability scanners, configures WAF rules, and schedules dependency audits. |
| **MLOps** | Model Pipelines | Builds containerized serving setups, tracks experiments, and sets up feature stores. |

---

## CLI Skill Management

Subscribers of **gptcode live** can interact with and manage these built-in skills directly via the `gt` binary:

```bash
# List all verified skills loaded in your session
gt skills list

# Synchronize and download a specific skill from the Live dashboard
gt skills install ruby

# Sync all subscription-entitled skills
gt skills install-all

# Inspect the architectural guidelines of a skill locally
gt skills show ruby
```

---

## Centralized Governance via gptcode live

For teams and enterprises, skills are not just local configurations; they are active policies. Through your **gptcode live** dashboard, administrators can:

1.  **Enforce Corporate Coding Standards**: Customize or disable specific built-in skills globally for the entire workspace.
2.  **Verify Compliance**: Reject commits and pull requests that fail validation against active DevOps and Security skill rules.
3.  **Deploy Proprietary Guidelines**: Upload internal team playbooks as custom skills, distributing them instantly to all active developer CLI binaries in real-time.
