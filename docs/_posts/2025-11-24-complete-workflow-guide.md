---
layout: post
title: "Complete Workflow: From Feature Idea to Implementation"
date: 2025-11-24
author: Jader Correa
description: "Master GPTCode's three-phase workflow: Research codebase context, plan implementation steps, and execute interactively or autonomously with built-in verification."
tags: [guides, workflow, best-practices, tutorial]
---

# Complete Workflow: From Feature Idea to Implementation

Learn how to use GPTCode's full workflow to go from a feature idea to working, tested code.

## The Three-Phase Workflow

GPTCode provides a structured approach to feature development:

**Phase 1: Research** → Understand your codebase  
**Phase 2: Plan** → Create detailed implementation steps  
**Phase 3: Implement** → Execute with verification (interactive or autonomous)

## Why This Workflow?

Traditional AI coding assistants give you code immediately. Sometimes that works. Often it doesn't because:

❌ No context about your codebase  
❌ No understanding of existing patterns  
❌ No incremental verification  
❌ No way to course-correct  

GPTCode's workflow[^1] solves this:

✅ Research phase builds context  
✅ Planning ensures coherent approach[^2]  
✅ Implementation is incremental and verified  
✅ You control the pace (interactive or autonomous)

## Quick Example

Let's add a "password reset" feature:

### 1. Research

```bash
gt research "How is user authentication currently implemented?"
```

GPTCode will:
- Search your codebase semantically
- Read and analyze relevant files
- Document current architecture
- Save findings to `~/.gptcode/research/`

### 2. Plan

```bash
gt plan "Add password reset feature with email verification"
```

GPTCode creates a detailed plan:
- Phase 1: Database changes (migration, columns)
- Phase 2: Email service (templates, sending)
- Phase 3: API endpoints (routes, validation)
- Phase 4: Tests (unit, integration, e2e)

Plan saved to `~/.gptcode/plans/2025-01-23-password-reset.md`

### 3. Implement (Interactive)

```bash
gptcode implement ~/.gptcode/plans/2025-01-23-password-reset.md
```

Walk through each phase:
```
─── Step 1/4: Database changes ───

Add reset_token and token_expiry columns to users table.
Create migration file...

Execute this step? [Y/n/q]: Y
✓ Step completed

─── Step 2/4: Email service ───
...
```

**Or Implement (Autonomous):**

```bash
gptcode implement ~/.gptcode/plans/2025-01-23-password-reset.md --auto
```

GPTCode executes everything:
- Runs each step
- Verifies with build + tests
- Retries on errors
- Creates checkpoints
- Completes autonomously

## Interactive vs Autonomous: When to Use Each

### Interactive Mode (Default)

Use when:
- 🎓 Learning unfamiliar codebase
- 🔒 Making sensitive/production changes
- 🤔 You want to understand each step
- 👀 Need to review before proceeding

**Pros:**
- Full control over execution
- See what changes before they happen
- Skip or quit at any point
- Learn as you go

**Cons:**
- Slower (requires manual confirmation)
- Can't walk away
- More active attention needed

### Autonomous Mode (--auto)

Use when:
- ✅ Plan is well-defined and reviewed
- 🚀 Want fast iteration
- 🤖 Trust your AI agent configuration
- 📦 Batch processing multiple features

**Pros:**
- Fully automated execution
- Verification at each step
- Error recovery with retry
- Checkpoint/resume support

**Cons:**
- Less visibility during execution
- Need to review changes after
- Requires good plan quality

## Real-World Tips

### For Best Results

1. **Always start with research** for unfamiliar areas
2. **Review and edit plans** before implementing
3. **Use interactive mode first**, then autonomous for iterations
4. **Review with `git diff`** after autonomous runs
5. **Commit incrementally** (one phase at a time is fine)

### When Plans Fail

**Interactive mode:**
- Quit, edit plan, restart
- Or continue anyway and fix manually

**Autonomous mode:**
- Automatic retry (3x default)
- Rollback to last checkpoint on failure
- Edit plan and `--resume` from checkpoint

### Good Plan Characteristics

✅ Clear, single-responsibility steps  
✅ Specific file paths mentioned  
✅ Test requirements for each phase  
✅ Incremental, verifiable changes  

❌ Vague "implement feature X"  
❌ Too many changes in one step  
❌ No verification criteria  

## Neovim Integration

All three phases work from Neovim:

```vim
" Phase 1: Research
<C-d>  " Open chat
> research: How does authentication work?

" Phase 2: Plan  
> plan: Add password reset feature

" Phase 3: Implement (autonomous)
:GPTCodeAuto
" Or: <leader>ca
```

## Cost Optimization

Using the full workflow actually **saves money**:

1. **Research** (one-time): ~10-50k tokens
2. **Plan** (one-time): ~20-100k tokens  
3. **Implement** (verified): ~100-500k tokens

**vs. Direct coding without context:**
- Multiple failed attempts
- Back-and-forth corrections
- Wasted tokens on wrong approaches
- Final cost: often 2-5x higher

**Example costs with Groq:**
- Research: $0.01-0.05
- Plan: $0.02-0.10
- Implement: $0.10-0.50
- **Total: $0.13-0.65 per feature**

Compare to 10+ coding attempts without planning: $1-3+ easily.

## Language Support

**Research & Plan:** Works with any language (language-agnostic)

**Implement verification:**
- ✅ Go
- ✅ TypeScript/JavaScript
- ✅ Python
- ✅ Elixir
- ✅ Ruby

Implementation itself works for any language (LLM-based), but build/test verification is language-specific.

## Try It Yourself

1. Pick a small feature to implement
2. Start with: `gt research "How does X work?"`
3. Create plan: `gt plan "Add Y feature"`
4. Implement interactively: `gptcode implement <plan>`
5. Review results, iterate if needed
6. Next time: use `--auto` for speed

## See Also

- [Complete Workflow Guide (docs)](../workflow-guide.md)
- [Cost Optimization Guide](2024-11-22-cost-tracking-optimization.md)
- [Groq Model Configs](2025-11-18-groq-optimal-configs.md)
- [Local Setup with Ollama](2024-11-19-ollama-local-setup.md)

---

## References

[^1]: Beck, K. (2003). *Test-Driven Development: By Example*. Addison-Wesley Professional. ISBN: 978-0321146533

[^2]: Fowler, M. (2018). *Refactoring: Improving the Design of Existing Code* (2nd ed.). Addison-Wesley Professional. ISBN: 978-0134757599

---

**Questions or issues?** [Open an issue on GitHub](https://github.com/jadercorrea/gptcode/issues)
