---
layout: post
title: "Optimal Groq Configurations: Technical Analysis & Profile Guide"
date: 2025-11-15
author: Jader Correa
description: "Deep technical analysis of Groq models with rigorously optimized profiles for budget and performance. Understand why router speed matters, editor output costs dominate, and 32B coding-focused models beat 70B generic ones."
tags: [configuration, groq, optimization, cost, profiles]
---

# Optimal Groq Configurations: Technical Analysis & Profile Guide

Groq offers blazing-fast inference speeds with their LPU technology. This guide provides **rigorously analyzed** configurations based on real model pricing, role-specific requirements, and actual I/O patterns.

## Understanding Agent Roles & I/O Patterns

Each agent has different I/O characteristics that dramatically affect cost:

| Agent | I/O Ratio | Critical Factor | Why |
|-------|-----------|-----------------|-----|
| **Router** | Input >> Output | **Speed** | Called most frequently, minimal output |
| **Query** | Balanced | **Reasoning** | Analyzes code, balanced I/O |
| **Editor** | Output >> Input (1:5) | **Output Cost** | Generates lots of code, output dominates |
| **Research** | Variable | **Tools + Quality** | Complex queries with external APIs |

### Key Insight: Editor Output Cost Dominates

For a typical editing session:
- Input: 1,000 tokens (prompt + context)
- Output: 5,000 tokens (generated code)

**Total cost** = (Input × $I) + (Output × 5 × $O)

This means **output price is 5x more important** than input price for editors.

## Budget Profile: Cost-Effective Configuration

Use: `gt profile use groq.budget`

```yaml
backend:
  groq:
    profiles:
      budget:
        agent_models:
          router: llama-3.1-8b-instant
          query: openai/gpt-oss-120b
          editor: qwen/qwen3-32b
          research: groq/compound
```

**Estimated cost**: $0.85/month for 3M tokens (~150k/day)

| Usage Level | Tokens/Month | Cost/Month |
|-------------|--------------|------------|
| Light | 1M (~50k/day) | $0.28 |
| **Moderate** | **3M (~150k/day)** | **$0.85** |
| Heavy | 5M (~250k/day) | $1.42 |

### Technical Rationale

**Router: llama-3.1-8b-instant** ($0.05/$0.08)
- 8B sufficient for routing decisions
- Fastest inference on Groq LPU
- Called most frequently → speed critical
- Cheapest option available

**Query: openai/gpt-oss-120b** ($0.15/$0.60)
- 120B parameters for deep reasoning
- Strong code comprehension
- Balanced I/O → reasonable cost

**Editor: qwen/qwen3-32b** ($0.29/$0.59) ← **KEY CHOICE**
- **Total cost**: $0.29 + ($0.59 × 5) = **$3.24/1M**
- **Coding-specialized**: Qwen series optimized for code
- **40% cheaper** than llama-3.3-70b ($4.54/1M total)
- 32B sufficient for most editing tasks
- **Why NOT llama-3.3-70b?** Generic "versatile" model costs more and isn't coding-focused

**Research: groq/compound** ($0.15/$0.60 base)
- GPT-OSS-120B + Llama 4 Scout internally
- Built-in tools: web search, code execution, browser
- Base pricing + tool costs ($5-8/1000 searches)
- **NOT free** - conservative estimate uses highest base price

## Performance Profile: Quality-First Configuration

Use: `gt profile use groq.performance`

```yaml
backend:
  groq:
    profiles:
      performance:
        agent_models:
          router: llama-3.1-8b-instant
          query: openai/gpt-oss-120b
          editor: moonshotai/kimi-k2-instruct
          research: groq/compound
```

**Estimated cost**: $2.41/month for 3M tokens (~150k/day)

| Usage Level | Tokens/Month | Cost/Month |
|-------------|--------------|------------|
| Light | 1M (~50k/day) | $0.80 |
| **Moderate** | **3M (~150k/day)** | **$2.41** |
| Heavy | 5M (~250k/day) | $4.01 |

### Technical Rationale

**Router: llama-3.1-8b-instant** ($0.05/$0.08) ← **Still fast!**
- **Why NOT llama-3.3-70b?** Speed matters more than size for routing
- 8B sufficient for intent classification
- 70B would add latency without meaningful benefit

**Query: openai/gpt-oss-120b** ($0.15/$0.60)
- Same as budget (already optimal)
- 120B parameters for reasoning
- No better option at this price point

**Editor: moonshotai/kimi-k2-instruct** ($1.00/$3.00) ← **Premium choice**
- **Total cost**: $1.00 + ($3.00 × 5) = **$16/1M**
- **5x more expensive** than budget, but worth it for:
  - Large context window (262K vs 131K)
  - Superior quality for complex refactoring
  - Better handling of multi-file edits
- **Why NOT llama-3.3-70b?** More expensive ($4.54/1M) and lower quality than Kimi K2

**Research: groq/compound** ($0.15/$0.60 base)
- Same as budget (best option with tools)
- Unified interface vs managing separate APIs

## Available Models Analysis

Real pricing from Groq API (verified Nov 2025):

| Model | Size | Input $/1M | Output $/1M | Best For |
|-------|------|------------|-------------|----------|
| llama-3.1-8b-instant | 8B | $0.05 | $0.08 | Router (speed) |
| openai/gpt-oss-safeguard-20b | 20B | $0.075 | $0.30 | Budget editing |
| meta-llama/llama-4-scout-17b | 17B | $0.11 | $0.34 | Tool use |
| openai/gpt-oss-120b | 120B | $0.15 | $0.60 | Query/reasoning |
| qwen/qwen3-32b | 32B | $0.29 | $0.59 | **Coding-focused** |
| llama-3.3-70b-versatile | 70B | $0.59 | $0.79 | Generic tasks |
| moonshotai/kimi-k2-instruct | Large | $1.00 | $3.00 | Premium quality |
| groq/compound | Composite | $0.15* | $0.60* | Research + tools |

*Compound: Base model + tool costs ($5-8/1000 searches, $0.18/hour code exec)

### Editor Cost Comparison (1:5 I/O ratio)

| Model | Input | Output×5 | **Total/1M** | Notes |
|-------|-------|----------|--------------|-------|
| qwen/qwen3-32b | $0.29 | $2.95 | **$3.24** | Coding-specialized |
| llama-3.3-70b | $0.59 | $3.95 | **$4.54** | Generic, 40% more expensive |
| moonshotai/kimi-k2 | $1.00 | $15.00 | **$16.00** | Premium, 5x more expensive |

## Key Insights

### 1. Router Should Stay Fast (Even in Performance Profile)

❌ **Wrong**: Use llama-3.3-70b for router in performance profile
✅ **Right**: Use llama-3.1-8b-instant even in performance

**Why?**
- Router is called most frequently
- 8B is sufficient for intent classification
- 70B adds latency without meaningful benefit
- Speed > size for this role

### 2. Editor Output Cost Dominates

❌ **Wrong**: Choose editor by parameter count
✅ **Right**: Calculate total cost with 1:5 I/O ratio

**Example:**
- llama-3.3-70b: $0.59 + ($0.79 × 5) = $4.54/1M
- qwen/qwen3-32b: $0.29 + ($0.59 × 5) = $3.24/1M

**Result**: 32B coding-focused model is 40% cheaper AND better for code

### 3. Compound Has Composite Pricing

❌ **Wrong**: "groq/compound is free"
✅ **Right**: Base model ($0.15/$0.60) + tool costs

**Tool costs:**
- Basic web search: $5/1000 requests
- Advanced search: $8/1000 requests
- Code execution: $0.18/hour
- Browser automation: $0.08/hour

**Still cost-effective**: Unified interface vs managing separate APIs

### 4. Coding-Specialized 32B > Generic 70B

❌ **Wrong**: Bigger model = better for code
✅ **Right**: Qwen series specialized for coding

**Budget profile**: qwen/qwen3-32b beats llama-3.3-70b:
- 40% cheaper
- Coding-focused architecture
- Sufficient parameters for editing

**Performance profile**: moonshotai/kimi-k2-instruct for complex tasks:
- Large context (262K)
- Superior quality
- Worth 5x cost premium

## Setting Up Profiles

### Option 1: Use Pre-configured Profiles

```bash
# Budget profile ($2-5/month)
gt profile use groq.budget

# Performance profile ($10-20/month)
gt profile use groq.performance

# Verify current profile
gt profile
```

### Option 2: Configure Manually

Edit `~/.gptcode/setup.yaml`:

```yaml
defaults:
  backend: groq
  profile: budget  # or performance

backend:
  groq:
    type: openai
    base_url: https://api.groq.com/openai/v1
    profiles:
      budget:
        agent_models:
          router: llama-3.1-8b-instant
          query: openai/gpt-oss-120b
          editor: qwen/qwen3-32b
          research: groq/compound
      performance:
        agent_models:
          router: llama-3.1-8b-instant
          query: openai/gpt-oss-120b
          editor: moonshotai/kimi-k2-instruct
          research: groq/compound
```

## Understanding Groq Compound

Compound is **not a single model** - it's a system that dynamically routes between:
- **GPT-OSS-120B**: Base reasoning model
- **Llama 4 Scout 17B**: Tool use specialist

### Built-in Tools
- Web search (basic & advanced)
- Code execution
- Browser automation
- Website visits

### Pricing Structure
**Base**: $0.15 input / $0.60 output (GPT-OSS-120B rate)

**Tools** (per-use charges):
- Basic web search: $5/1000 requests
- Advanced search: $8/1000 requests
- Visit website: $1/1000 visits
- Code execution: $0.18/hour
- Browser automation: $0.08/hour

**Conservative estimate**: Use $0.15/$0.60 as baseline + monitor tool usage

### Why Use Compound?
- Unified interface (no separate API management)
- Automatic model routing
- Integrated tools for research
- Cost-effective vs separate services

## Migration Guide

### From Old Configs

If you have old profile configurations, update them:

❌ **Old** (suboptimal):
```yaml
agent_models:
  router: llama-3.3-70b-versatile  # too slow
  editor: llama-3.3-70b-versatile  # not coding-focused
```

✅ **New** (optimized):
```yaml
agent_models:
  router: llama-3.1-8b-instant     # faster
  editor: qwen/qwen3-32b           # coding-specialized, cheaper
```

### Update Model Catalog

After changing profiles, update catalog to see available models:

```bash
gt model update --all
gt model list groq
```

## Monitoring & Optimization

### Check Current Configuration
```bash
gt profile                    # Show current backend and profile
gt profile show groq.budget   # Show specific profile details
```

### Monitor Usage
- Visit [console.groq.com](https://console.groq.com) for usage stats
- Track which agent is called most (usually router)
- Identify bottlenecks in your workflow

### Optimization Tips

1. **Start with budget**, upgrade specific agents as needed
2. **Router should always be fast** (llama-3.1-8b-instant)
3. **Editor output cost dominates** - calculate with 1:5 ratio
4. **Query balance** - gpt-oss-120b is sweet spot
5. **Compound is not free** - monitor tool usage

## Common Mistakes

### ❌ Using 70B for Router
**Problem**: Adds latency without benefit
**Fix**: Use llama-3.1-8b-instant even in performance profile

### ❌ Choosing Editor by Parameters
**Problem**: Ignores output cost (5x more important)
**Fix**: Calculate total cost, choose coding-specialized models

### ❌ Assuming Compound is Free
**Problem**: Unexpected tool costs
**Fix**: Budget for $0.15/$0.60 base + $5-8/1000 searches

### ❌ Generic 70B Over Specialized 32B
**Problem**: Higher cost, not coding-focused
**Fix**: qwen/qwen3-32b beats llama-3.3-70b for budget editing

## Cost Comparison

### Groq vs Claude Pro Max

**Claude Pro Max**: $200/month for ~4.8M tokens (~800 prompts/session)

**Groq** (5M tokens/month, ~250k/day):
- **Budget**: $1.42/month (~**99% cheaper**)
- **Performance**: $4.01/month (~**98% cheaper**)

### Why So Cheap?

1. **Groq LPU efficiency**: Hardware-level optimization
2. **Open-source models**: No model training costs passed to users
3. **Competitive pricing**: Groq subsidizing to gain market share
4. **Smart agent routing**: 40% of calls use cheapest model (router)

### Usage Breakdown (3M tokens/month typical)

**Budget Profile** ($0.85/month):
- Router (40%): $0.06 - llama-3.1-8b-instant
- Query (30%): $0.34 - openai/gpt-oss-120b
- Editor (25%): $0.40 - qwen/qwen3-32b
- Research (5%): $0.06 - groq/compound

**Performance Profile** ($2.41/month):
- Router (40%): $0.06 - llama-3.1-8b-instant (same)
- Query (30%): $0.34 - openai/gpt-oss-120b (same)
- Editor (25%): $1.95 - moonshotai/kimi-k2-instruct (5x more)
- Research (5%): $0.06 - groq/compound (same)

**Key insight**: Editor accounts for 46% of budget cost but 81% of performance cost.

---

*Questions or better configurations? Share on [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)!*
