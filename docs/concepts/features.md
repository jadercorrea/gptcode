---
layout: default
title: Core Agent Features
description: Built-in features and execution capabilities of the pre-compiled Live CLI Agent. Learn how the local runner orchestrates multi-agent tasks, optimizes context, and integrates with the Live dashboard.
---

# Core Agent Features

The **gptcode CLI agent** (`gt`) is a pre-compiled, high-performance binary engineered to run complex autonomous coding loops directly on your machine or host server. It combines localized intelligence with active control policies streamed from your **gptcode live** subscription.

---

## 1. High-Performance Multi-Agent Architecture

The cornerstone of the `gt` agent is its specialized, multi-stage pipeline. When you issue a task, the agent orchestrates four distinct agent personalities to execute the job safely and verify correctness locally.

```
┌─────────────────────────────────────────────────────────────────┐
│                          You run: gt do                         │
├───────────────┬───────────────┬────────────────┬────────────────┤
│  1. ANALYZER  │   2. PLANNER  │   3. EDITOR    │  4. VALIDATOR  │
│ Scans code &  │ Creates minimal│ Modifies files │ Executes tests │
│ dependencies  │  diff target  │  under sandbox │  and compiles  │
└───────────────┴───────────────┴────────────────┴────────────────┘
```

### Flagship Commands
```bash
# Execute a task autonomously
gt do "add email notification hook to user schema"

# Require manual approval before applying modifications
gt do "refactor legacy database model" --supervised

# View real-time model selections and cost calculations
gt do "optimize critical loop" --verbose
```

### Flags Reference
*   `--supervised`: Forces the agent to halt and request human approval via the terminal or Live Dashboard before applying file modifications.
*   `--interactive`: Prompts the user when the local intent router encounters ambiguous commands.
*   `--dry-run`: Generates and displays the execution plan without making any changes.
*   `--max-attempts N`: Sets the maximum self-healing retries if validation fails (default: 3).

---

## 2. Active Validation & Safety Guardrails

Security is baked directly into the pre-compiled binary. The agent cannot run wild on your system; all actions are bound by strict local and remote policies.

*   **File Sandbox Validation**: The **Editor** agent is strictly confined. It can *only* modify files explicitly approved by the **Planner** agent during the plan phase. The binary rejects any attempt to touch unauthorized system directories, hidden scripts, or cross-project files.
*   **Self-Healing Test Cycles**: The **Validator** agent automatically compiles code, runs language-specific tests, evaluates linters, and retries if compilation or tests fail. If self-healing fails after maximum attempts, `gt` automatically rolls back the workspace to its clean git HEAD state.
*   **Active Interception Policies**: Centrally managed rules from your **gptcode live** dashboard are pushed to the binary in real time. If the agent attempts a command blocked in your organization’s `agentops.yml`, the process is actively aborted.

---

## 3. High-Speed Local ML Intelligence

To minimize latency and subscription API costs, `gt` embeds lightweight local ML models compiled directly into the Go binary.

*   **Intent Classification (1ms)**: Instantly routes user inputs (categorizing as code query, file edit, documentation research, or structural review) in **1ms** instead of waiting for a 500ms cloud LLM call. This reduces routing API costs to zero.
*   **Complexity Detection**: Automatically analyzes the task structure and local dependency trees to trigger **Guided Mode** (multi-agent orchestration) for complex modifications, or fast single-shot routing for basic questions.
*   **Heuristic Threshold Configuration**:
    ```bash
    # View current ML thresholds
    gt config get defaults.ml_intent_threshold
    
    # Adjust sensitivity via CLI config
    gt config set defaults.ml_intent_threshold 0.8
    ```

---

## 4. Intelligent Context Selection (PageRank Graph)

To avoid feeding massive codebases into LLM context windows (which causes high costs and model confusion), the pre-compiled binary builds a localized import graph.

*   **Codebase PageRank Analysis**: Analyzes import syntax and structural dependencies to build a real-time dependency graph of your workspace.
*   **Focused 1-Hop Context**: When you ask a question or request a change, `gt` runs keyword matching combined with PageRank weights to extract the top files and their direct dependencies (1-hop neighbors).
*   **Token Optimization**: Automatically reduces typical context size by 5x (e.g., packing a 100k-token repository context into a highly focused 20k-token payload), maintaining high model accuracy.
*   **Graph Debugging**:
    ```bash
    # Force rebuild of localized dependency cache
    gt graph build
    
    # Query dependency weights for a concept
    gt graph query "authentication"
    ```

---

## 5. Centralized Cost & Model Governance

Although the compiled agent executes code locally with total privacy, its model usage and telemetry are managed centrally via **gptcode live**.

*   **Bring Your Own Keys (BYOK)**: Connect your own corporate API keys (OpenAI, Anthropic, DeepSeek, Groq, OpenRouter) safely through the Live Control Plane.
*   **Centralized Budget Caps**: Configure hard spending limits per user, repository, or workspace. If a local execution loop gets stuck, the Live policy engine triggers a remote `SIGTERM` interrupt automatically.
*   **Automatic PII Redactor**: Before any logs, files, or tokens are streamed to your Live visualization dashboard, an embedded regex and ML-scrubbing pipeline redacts API keys, passwords, database URLs, and email addresses.

---

## 6. Local Developer Ecosystem

The compiled agent fits natively into advanced developer configurations.

*   **Neovim IDE Integration**: Seamless floating terminal chats, LSP context synchronization, tree-sitter integration, and profile management interfaces (`<C-d>` and `<C-m>`).
*   **Interactive TDD Runner**: Rapidly iterate using a test-driven development workflow (`gt tdd`), building robust coverage blocks before writing code.
*   **Operational DevOps Commands**: Execute tasks and DevOps scripts using `gt run [task]`, which supports direct command REPLs, output referencing, and environment management variables.
