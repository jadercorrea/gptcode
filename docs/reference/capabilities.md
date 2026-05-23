---
layout: default
title: Agent Capabilities & Technical Constraints
description: Official capabilities guide for the compiled gptcode Live CLI Agent. Detailed specification of supported autonomous loops, git operations, migrations, and test validations.
permalink: /reference/capabilities/
---

# Agent Capabilities & Technical Constraints

The **gptcode CLI agent** is a highly capable autonomous execution runner compiled for secure, on-premise or hosted developer systems. This reference documents the production capabilities, workflows, and language support built directly into the active binary bundle.

---

## 1. Autonomous Task & Issue Resolution

The compiled binary can execute end-to-end coding tasks autonomously, managing context selection, plan construction, file modifications, and local validations securely.

### Supported Workflows
*   **Context Parsing**: Securely analyzes task prompts and extracts requirements locally.
*   **Automatic File Discovery**: Locates relevant files using embedding-free, high-speed local PageRank algorithms.
*   **Targeted File Modification**: Constrains the Editor agent to modify *only* pre-approved files in the workspace.
*   **Linter & Build Verifications**: Automatically runs local compilers (`go build`, `npm run build`, etc.) and linters to resolve simple syntax issues or warning violations.
*   **Test-Suite Validation**: Executes local test runners and analyzes feedback logs to auto-heal test failures.
*   **Clean Workspace Rollback**: Automatically reverts staged file edits if an execution loop fails or exceeds safety cost thresholds.

### Language Support Matrix

| Category | Go | TypeScript/JS | Python | Ruby | Elixir | Rust |
|:---|:---:|:---:|:---:|:---:|:---:|:---:|
| **Build Compiling** | Yes | Yes | N/A | N/A | Yes | Yes |
| **Linter Execution** | `golangci-lint` | `eslint`, `tsc` | `mypy`, `ruff` | `rubocop` | `credo` | `cargo clippy` |
| **Test Verification** | `go test` | `jest`, `npm test` | `pytest` | `rspec` | `mix test` | `cargo test` |
| **Code Coverage** | Yes | Yes (Jest) | Yes | Yes (SimpleCov) | Yes | Yes |
| **Security Scanning** | `govulncheck` | `npm audit` | `safety` | `bundle-audit` | `mix vulns` | `cargo audit` |

---

## 2. Advanced Code Refactoring & Migrations

Unlike standard passive AI completions, the `gt` agent features dedicated structural refactoring commands designed to coordinate complex multi-file codebase updates.

### A. Zero-Downtime Database Migrations (`gt evolve`)
Generates multi-phase, backward-compatible schema migration plans:
*   **Phase 1**: Adds nullable structures or temporary database columns.
*   **Phase 2**: Coordinates backfilling jobs and data migrations.
*   **Phase 3**: Applies non-null constraints and cleans up legacy tables.
*   **Operational safety**: Creates matching down-migration rollback scripts automatically.

### B. API Route Coordination (`gt refactor api`)
*   Scans routing layers and controller files to detect changes.
*   Generates or updates handler functions following project-specific REST/GraphQL conventions.
*   Constructs and pairs corresponding integration test cases.

### C. Signature Refactoring (`gt refactor signature`)
*   Locates target function and method definitions across your codebase.
*   Intelligently updates call sites across all importing files, resolving parameters, types, and contexts.
*   Prevents compilation errors by validating the complete build post-refactor.

### D. Automated Security Patching (`gt security scan --focus`)
*   Scans dependencies and structures for CVEs using language-specific tools.
*   Formulates safe version upgrade strategies.
*   Generates prompt-driven fixes to remediate vulnerability vectors (e.g., sanitizing SQL inputs, enforcing TLS configurations).

---

## 3. High-Fidelity Test & Mock Generation

Ensure high test coverage and enforce robust regression protection using local compilation-verified test generators.

*   **Unit Test Generation (`gt gen test <file>`)**: Analyzes internal logic branch paths and writes matching unit test files. The agent automatically compiles and runs the tests, resolving mock imports until the suite passes.
*   **Integration Test Generation (`gt gen integration <pkg>`)**: Constructs robust endpoint and boundary tests, validating database transactions or HTTP network cycles.
*   **Local Mock Generation (`gt gen mock <file>`)**: Scans interfaces and declarations to output conforming mock structures and helper stubs automatically.
*   **Coverage Gap Identification (`gt coverage`)**: Analyzes local coverage reports to locate uncovered code blocks, automatically spawning child agents to write tests targeting those specific lines.

---

## 4. Advanced Git & Conflict Operations

The pre-compiled agent manages branch histories and automates tedious git workflows safely on your behalf.

*   **AI-Powered Conflict Resolution (`gt merge resolve`)**: Detects conflicts, parses standard three-way merge markers, and utilizes high-performance models to merge changes, keeping business logic intact.
*   **Automated Bisecting (`gt git bisect <good> <bad>`)**: Runs automated bisect scripts across git history, executing your test suites at each step to pinpoint the exact commit that introduced a regression.
*   **Rebasing & Cherry-Picking (`gt git rebase`, `gt git cherry-pick`)**: Performs complex history operations, automatically resolving intermediate merge conflicts and verifying code compilation at each applied commit.
*   **Interactive History Squashing (`gt git squash`)**: Squashes commit ranges and drafts professional, Conventional-Commit-compliant commit descriptions based on file diff analyses.

---

## 5. Architectural Constraints & Limitations

To guarantee predictable, high-value outcomes, the pre-compiled agent maintains clear boundary limitations:

*   **Language-Specific Features**: Advanced refactoring engines (e.g., `gt refactor breaking`, `gt evolve`) require structured code parsing and are highly optimized for Go, TypeScript, and Python.
*   **Network Isolation**: Local execution is sandboxed. The agent cannot fetch remote dependencies, download arbitrary packages, or execute unverified commands unless explicitly permitted in your `agentops.yml` policy.
*   **Database Constraints**: Automated database migrations are pre-configured to output PostgreSQL and MySQL syntax; custom NoSQL or key-value schemas require user-supervised prompts.
*   **Conflict Context Limits**: The merge conflict resolver is optimized for conflicts spanning up to 300 lines; extremely large, cross-module structural conflicts require developer intervention.
