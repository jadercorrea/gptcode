---
layout: post
title: "Agent Routing vs. Tool Search: Two Paths to 85% Context Reduction"
date: 2025-12-01
author: Jader Correa
description: "Anthropic's Tool Search and GPTCode's Agent Routing solve the same problem differently. Here's what we learned."
tags: [architecture, ml, agents, context-optimization]
---

# Agent Routing vs. Tool Search: Two Paths to 85% Context Reduction

Anthropic just released [advanced tool use features](https://www.anthropic.com/engineering/advanced-tool-use) that achieve 85% context reduction through on-demand tool discovery.

GPTCode already does this—but with a fundamentally different architecture.

## The Problem: Token Bloat

**Anthropic's scenario**: 
- 58 tools across 5 MCP servers (GitHub, Slack, Jira, Sentry, Grafana)
- **55,000 tokens** consumed before any work begins
- Add more servers → 72,000+ tokens
- Real production systems: **134,000+ tokens**

**GPTCode's scenario**:
- Large codebase with 500+ files
- **100,000+ tokens** of potential context
- Need to identify relevant files for each task
- Need to execute multi-step changes safely

Same fundamental problem: **too much context drowns the signal**.

## Solution #1: Discovery

### Anthropic: Tool Search Tool

Instead of loading all 58 tools upfront, Anthropic defers most tools and discovers them on-demand:

1. Load only a search tool (~500 tokens)
2. When Claude needs GitHub capabilities, search for "github"
3. Load only `github.createPullRequest` and `github.listIssues` (~3K tokens)
4. Leave the other 56 tools deferred

**Result**: 85% reduction (77K → 8.7K tokens)

**Trade-off**: Adds search latency to every task

### GPTCode: Agent Routing + Semantic Filtering

GPTCode doesn't use a tool search. Instead, it routes to **specialized agents**:

```bash
gt do "add authentication"
  ↓
ML Classifier (1ms) → OrchestratedMode
  ↓
Router Agent → Analyzer Agent → Planner Agent → Editor Agent → Validator Agent
```

Each agent has a **narrow, focused capability set**:
- **Analyzer**: File scanning, dependency graphs, PageRank
- **Planner**: Implementation planning, success criteria
- **Editor**: Code generation, file editing
- **Validator**: Test running, lint checking

Context is pre-filtered via **PageRank + dependency analysis**:
- 100K codebase → identify 14 relevant files
- Load only those files into Planner context

**Result**: 80% reduction (100K → 20K tokens)

**Trade-off**: Agents must be designed upfront (less flexible than dynamic tool discovery)

## Solution #2: Code Orchestration

Both systems keep intermediate results out of the LLM's context.

### Anthropic: Programmatic Tool Calling

Claude writes Python code that orchestrates tools:

```python
# Claude writes this orchestration code
team = await get_team_members("engineering")
expenses = await asyncio.gather(*[
    get_expenses(user_id, "Q3") for user_id in team
])

# Only final result enters Claude's context
exceeded = [
    user for user in team 
    if sum(expenses[user]) > budget[user["level"]]
]
print(json.dumps(exceeded))  # Just 3 people
```

**Impact**: Process 2,000+ expense line items, but only 3 results enter context.  
**Token savings**: 37% reduction (43,588 → 27,297 tokens on complex tasks)

### GPTCode: Maestro Pipeline

GPTCode's autonomous mode (`gt do`) implements orchestration as an **agent pipeline**:

```
Analyzer (scans 100 files)
    ↓ outputs: dependency graph only
Planner (creates 5-step plan)
    ↓ outputs: file list + success criteria only
Editor (edits 3 files)
    ↓ outputs: diffs only
Validator (runs 50 tests)
    ↓ outputs: pass/fail summary only
```

**Impact**: 100+ files analyzed, but only 3 file paths + test results in final context.

The **Validation Gate** ensures intermediate bloat never enters the editor:
- Planner outputs: `[auth/index.ts, middleware/jwt.ts, tests/auth.test.ts]`
- Editor receives: Only those 3 files
- Validator outputs: `12/12 tests passing`

**Key difference**: GPTCode's orchestration is **structural** (agent handoffs), while Anthropic's is **programmatic** (LLM-written code).

## Solution #3: Parameter Accuracy

This is where Anthropic's innovation shines—and where GPTCode has room to improve.

### Anthropic: Tool Use Examples

JSON Schema defines structure, but not **usage patterns**. Anthropic adds concrete examples:

```json
{
  "name": "create_ticket",
  "input_schema": {
    "properties": {
      "title": {"type": "string"},
      "due_date": {"type": "string"},
      "reporter": {
        "properties": {
          "id": {"type": "string"},
          "contact": {"properties": {...}}
        }
      }
    }
  },
  "input_examples": [
    {
      "title": "Login page returns 500",
      "priority": "critical",
      "due_date": "2024-11-06",
      "reporter": {
        "id": "USR-12345",
        "contact": {"email": "jane@acme.com"}
      }
    },
    {
      "title": "Add dark mode",
      "labels": ["feature-request"],
      "reporter": {"id": "USR-67890"}
    }
  ]
}
```

From these examples, Claude learns:
- Date format: `YYYY-MM-DD`
- ID convention: `USR-XXXXX`
- When to include optional fields (contact info for critical bugs, not for features)

**Result**: 72% → 90% accuracy on complex parameters

### GPTCode: Validation + Feedback (Current)

GPTCode currently relies on:
- **File validation**: Prevents unintended file creation
- **Success criteria**: Test-based verification
- **Auto-recovery**: Switches models when validation fails

This catches errors **after the fact**, but doesn't prevent them upfront.

### What We're Adding: Concrete Examples in Prompts

We're borrowing Anthropic's best idea and adding explicit examples to each agent:

**Analyzer agent**:
```
Example: Analyzing Go authentication code
Input: "How does user authentication work?"
Output:
- Files found: [auth/index.go, middleware/jwt.go, handlers/login.go]
- Dependencies: jwt.go → auth.go → handlers.go
- PageRank scores: [0.82, 0.71, 0.45]
```

**Planner agent**:
```
Example: Plan for adding authentication
Task: "add user authentication"
Output:
Phase 1: Core authentication
  - Create: auth/handler.go (login, logout, verify)
  - Modify: server.go (add auth middleware)
  
Phase 2: JWT implementation  
  - Create: auth/jwt.go (sign, verify tokens)
  - Modify: middleware/ (auth middleware)

Success criteria:
  - Tests pass: auth_test.go
  - Lints clean
```

**Editor agent**:
```
Example: File edit using search/replace
File: auth/handler.go
Change: Add JWT verification

<<<<<<< SEARCH
func VerifyToken(token string) bool {
    // TODO: implement
    return false
}
=======
func VerifyToken(token string) (*Claims, error) {
    claims := &Claims{}
    parsed, err := jwt.ParseWithClaims(token, claims, keyFunc)
    if err != nil || !parsed.Valid {
        return nil, err
    }
    return claims, nil
}
>>>>>>> REPLACE
```

**Expected impact**: 10-20% accuracy improvement, fewer retry loops.

## Conclusion: Architecture Matters

Advanced tool use isn't just about features—it's about **system design**.

|  | Anthropic | GPTCode |
|---|---|---|
| **Philosophy** | Single mega-agent + dynamic discovery | Multi-agent specialization |
| **Discovery** | Tool Search (on-demand) | Agent Routing (1ms classifier) |
| **Orchestration** | Programmatic (Python scripts) | Structural (pipeline) |
| **Context Reduction** | 85% (77K → 8.7K) | 80% (100K → 20K) |
| **Flexibility** | High (add tools dynamically) | Medium (agents designed upfront) |
| **Latency** | +search overhead per task | Minimal (1ms routing) |

Both achieve ~85% context reduction through different paths:
- **On-demand flexibility** (Anthropic) vs. **Upfront specialization** (GPTCode)
- **Dynamic tool discovery** vs. **Static agent pipeline**

We're adopting Anthropic's **Tool Use Examples** pattern while keeping our fast, specialized architecture.

## Try It

```bash
gt do "add user authentication"
# 1ms routing → Analyzer → Planner → Editor → Validator
# 100K codebase → 20K relevant context → 3 files modified
# Total cost: $0.000556 (vs. $0.01+ with full context)
# Auto-retry with model switching if tests fail
```

---

## References

- Anthropic. (2025). [Advanced Tool Use](https://www.anthropic.com/engineering/advanced-tool-use). Engineering Blog.

---

*Have questions about agent routing vs tool search? Join our [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)*

## See Also

- [ML-Powered Intelligence](2025-11-22-ml-powered-intelligence) - GPTCode's ML capabilities
- [Intelligent Auto-Recovery](2025-11-26-intelligent-auto-recovery) - Auto-recovery system
- [Context Engineering](2025-11-14-context-engineering-for-real-codebases) - Making AI work in real codebases
