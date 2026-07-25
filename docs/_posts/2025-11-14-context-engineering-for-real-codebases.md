---
layout: post
title: "Context Engineering: Making AI Work in Real Codebases"
date: 2025-11-14
author: Jader Correa
description: "Learn context engineering principles to make AI work effectively in production codebases. Proper context management enables handling 300k+ LOC repos with today's models."
tags: [context-management, architecture, performance, best-practices]
---

# Context Engineering: Making AI Work in Real Codebases

In the [previous post]({% post_url 2025-11-13-why-gptcode %}), we talked about **why** GPTCode exists—making AI coding assistance affordable. Now let's talk about **how** to actually make it work in production codebases.

## The Real Problem

The [Stanford study on AI's impact on developer productivity](https://www.youtube.com/watch?v=tbDDYKRFjhk) found something concerning:

1. A lot of "extra code" shipped by AI tools ends up getting reworked the next week
2. AI works well for greenfield projects but struggles with large established codebases

Sound familiar? The common responses are:
- "Too much slop"
- "Doesn't work in big repos"
- "Maybe someday when models are smarter..."

**But here's the thing**: You can get really far with today's models if you embrace core context engineering principles.

## What's Actually Possible

Recent experiments show that proper context management enables AI to handle:
- 300k+ LOC codebases
- Complex system changes (cancellation support, WASM compilation)
- Week-long features shipped in a day
- Code that passes expert review

This isn't about smarter models. It's about **context engineering**—the art of managing what information the LLM sees and when.

## Understanding Context Windows

LLMs are **stateless functions**. The only thing affecting output quality (without training new models) is **input quality**.

At any given turn, a coding agent is:
```
Context Window In → Next Step Out
```

That's it. The contents of your context window are **the only lever** you have.

### What Eats Context?

- Searching for files
- Understanding code flow
- Applying edits
- Test/build logs
- Large JSON responses from tools

All of these flood the context window with noise.

### Optimize For

1. **Correctness**: No wrong information
2. **Completeness**: All relevant information
3. **Compactness**: Minimal noise

Or as one equation:

```
Output Quality ∝ (Correctness × Completeness) / Noise
```

### The Golden Rule

> You only have ~170k tokens of context.
> Use as little as possible.
> The more you use, the worse the outcomes.

## The Naive Approach (Don't Do This)

Most people use AI coding tools like a chatbot:

1. Chat back and forth
2. Vibe your way through
3. Hit context limit or give up
4. Start over with "try again but use XYZ approach"

This fills context with noise and gets you stuck in loops.

## Better: Intentional Compaction

**Compaction** means distilling context into structured artifacts.

When context fills up, pause and ask:

> "Write everything we did so far to progress.md. Note:
> - The end goal
> - The approach we're taking
> - Steps completed
> - Current state/blockers"

Start a fresh session with this compact summary.

### What Good Compaction Looks Like

```markdown
## Goal
Add user authentication with JWT tokens

## Approach
1. Create User model with bcrypt password hashing
2. Add JWT generation/validation middleware
3. Protect routes with auth middleware

## Progress
- [x] User model created with tests
- [x] Password hashing working
- [ ] Currently: JWT middleware failing validation

## Current Issue
Token signature verification fails with RS256.
Need to check if we're using correct public key format.
```

This is 10 lines vs 1000+ lines of chat history.

## Even Better: Frequent Intentional Compaction

**Design your entire workflow around context management.**

Keep utilization in the 40-60% range. Split work into phases:

### 1. Research

Understand the codebase and problem:
- Which files are relevant?
- How does information flow?
- What are potential solutions?

Output: Compact research document with key findings.

### 2. Plan

Create precise implementation steps:
- Exact files to edit
- Specific changes per file
- Testing/verification at each phase

Output: Step-by-step plan with acceptance criteria.

### 3. Implement

Execute the plan phase by phase:
- One phase at a time
- Verify before moving on
- Compact progress back into plan

Output: Working, tested code.

## Why This Works in GPTCode

GPTCode's multi-agent architecture is designed around this principle:

**Router Agent** (8B model)
- Fast intent classification (~840 TPS)
- Minimal context needed for routing
- Routes to appropriate specialized agent

**Query Agent** (reasoning model)
- Research and codebase analysis[^1]
- Reads files, searches patterns
- Compacts findings into structured output
- Fresh context for each analysis

**Editor Agent** (code-specialized model)
- Receives focused context from query
- Implements changes incrementally
- Can use larger context models when needed

**Research Agent** (with web tools)
- External documentation lookup
- API reference search
- Summarizes findings separately from main work
- Keeps noise out of implementation context

**Key insight**: Each agent starts with a **clean, focused context** containing only what it needs for its specific task. No agent sees the full chat history—only relevant information.

## Human Leverage: Where to Focus

A bad line of code = 1 bad line
A bad line in a **plan** = 100s of bad lines
A bad line in **research** = 1000s of bad lines

**Focus human review on high-leverage artifacts:**

1. Review research documents (highest leverage)
2. Review implementation plans (medium leverage)
3. Review code (lowest leverage, but still important)

With this approach:
- You can't read 2000 lines of code daily
- But you **can** read 200 lines of a plan
- And you **can** steer research to focus on what matters

## Mental Alignment

The biggest problem with AI-generated code isn't correctness—it's **losing touch with your codebase**.

When AI ships 2000-line PRs daily, you start losing mental alignment with:
- What your product does
- How systems work
- Why decisions were made

Research/Plan/Implement artifacts solve this:
- Plans keep everyone aligned on changes
- Research documents explain the "why"
- You can quickly learn unfamiliar parts of the codebase

## Practical Tips for GPTCode

### Start With Focused Commands
```bash
# Research phase - understand the codebase
gt research "how does user auth work in this codebase"
# Read the output, steer if needed

# Plan phase - create structured plan
gt plan "add password reset via email"
# Review the plan before implementing

# Implement phase - execute the plan
gptcode implement ~/.gptcode/plans/2024-11-15-password-reset.md
# Note: Implementation reads the plan and executes phase by phase
```

**Each command starts with fresh context**, avoiding the context pollution of long chat sessions.

### Use Different Models for Different Tasks

GPTCode lets you assign specialized models to each agent role:

```yaml
backend:
  groq:
    agent_models:
      router: llama-3.1-8b-instant      # Speed: 840 TPS
      query: llama-3.3-70b-versatile    # Reasoning: 70B params
      editor: llama-3.3-70b-versatile   # Coding: versatile
      research: groq/compound           # Tools: web search
```

**Why this works:**
- Router needs speed, not depth → use small/fast model
- Query needs comprehension → use reasoning model
- Editor needs code quality → use specialized coding model
- Research needs tools → use model with web search

Each agent gets the **right tool for its job**, not one-size-fits-all.

### Keep Context Tight

If you notice responses getting worse or repetitive:

1. **Save your progress**: Write summary to a file
2. **Exit current session**: Start fresh
3. **Resume with context**: Load the compact summary

GPTCode's command-based workflow naturally encourages this:
- `gt research` → outputs findings
- `gt plan` → reads findings, outputs plan  
- `gptcode implement` → reads plan, outputs code

Each step is **independently verifiable** and **resumable**.

### Incremental Verification

Don't try to do everything in one go:

```bash
# Step 1: Understand what needs to change
gt research "payment processing flow"

# Step 2: Create detailed plan
gt plan "add Stripe webhook handling"
# Review plan - does it make sense?

# Step 3: Implement incrementally
gptcode implement plan.md
# Review changes - does code match plan?
```

This workflow gives you **multiple checkpoints** to catch issues early, when they're cheap to fix.

## This Is Not Magic

You still need to:
- **Engage deeply** with the task
- **Review** research and plans
- **Steer** when things go wrong
- **Understand** the changes

There's no magic prompt that solves everything. But proper context engineering makes AI **actually useful** for hard problems.

## What Works

With this approach, GPTCode can:
- Work in brownfield codebases (not just toys)
- Solve complex problems (not just CRUD)
- Produce quality code (not slop)
- Maintain mental alignment (not black box)

And do it affordably:
- Groq: $2-5/month typical usage
- Ollama: $0/month (fully local)

## What's Next

In future posts we'll cover:
- Optimal model configurations for different project sizes
- Setting up local Ollama for zero-cost coding
- Advanced prompting techniques for TDD

But the foundation is always the same: **manage your context window like your productivity depends on it**—because it does.

---

## References

[^1]: Lewis, P., Perez, E., Piktus, A., et al. (2020). Retrieval-augmented generation for knowledge-intensive NLP tasks. *NeurIPS 2020*. https://arxiv.org/abs/2005.11401

---

*Have questions about context engineering? Join the discussion in [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)*
