---
layout: post
title: "Intelligent Model Selection: Cost-Optimized AI That Learns"
date: 2025-12-02
author: Jader Correa
description: "Intelligent model selection that automatically chooses the best model based on availability, cost, context window, and speed. Learn from usage patterns and optimize costs."
tags: [features, ml, cost-optimization, intelligence]
---

# Intelligent Model Selection: Cost-Optimized AI That Learns

In previous posts, we discussed [context engineering]({% post_url 2025-11-14-context-engineering-for-real-codebases %}) and [agent routing]({% post_url 2025-12-01-agent-routing-vs-tool-search %}). But there's another critical challenge: **choosing the right model for each task**.

## The Problem: Cloud AI Costs Add Up Fast

When building an AI coding assistant, one of the biggest challenges is cost management. Traditional approaches use fixed model assignments:
- Planning? Always use Model X
- Editing? Always use Model Y
- Review? Always use Model Z

This rigid approach has several problems:
1. **Ignores availability** - What if Model X hits rate limits?
2. **Wastes money** - Why use expensive models for simple tasks?
3. **No learning** - Repeats mistakes with failing models
4. **Manual management** - Users must constantly tweak configs

## The Solution: Multi-Dimensional Model Selection

GPTCode now features an intelligent model selection system that **automatically** chooses the best model for each action based on:

### 1. Availability (Highest Priority)
```
Score penalties:
- 90%+ rate limit usage: -50 points
- Recent errors: -30 points
```

The system tracks daily usage per model and automatically switches to fallback providers when limits are reached.

### 2. Cost Optimization
```
Free models (OpenRouter): No penalty
$0.30/1M tokens: -9 points
$3.00/1M tokens: -90 points
```

OpenRouter's free models (Gemini 2.0 Flash, Llama 3.2 3B, etc.) are prioritized, keeping costs near zero for most workloads.

### 3. Context Window
```
1M tokens: +100 bonus
128k tokens: +12.8 bonus
8k tokens: +0.8 bonus
```

Larger context windows handle complex refactorings better, so they get scoring bonuses.

### 4. Speed
```
150 tokens/sec: +7.5 bonus
50 tokens/sec: +2.5 bonus
```

Faster models improve developer experience with quick responses.

## How It Works

### Automatic Usage Tracking

Every LLM call records:
- Backend and model used
- Success/failure status
- Error messages
- Token usage (input/output/cached)

Data stored in `~/.gptcode/usage.json`:
```json
{
  "2025-12-02": {
    "openrouter/gemini-2.0-flash-exp:free": {
      "requests": 47,
      "input_tokens": 125000,
      "output_tokens": 8500,
      "cached_tokens": 89000,
      "last_error": null
    }
  }
}
```

### Elegant Stats Dashboard

```bash
gptcode stats
```

Output:
```
────────────────────────────────────────────────────────────
  Usage Statistics

  Period:              All Time
  Total Requests:      47
  Success Rate:        100.0%

  Token Usage
  Input Tokens:        125.0k
  Output Tokens:       8.5k
  Cached Tokens:       89.0k (71.2% cache hit)

  💡 Cache savings: 89.0k tokens, reducing costs

  Model Usage          Requests  Status
  ────────────────────────────────────────────────────────
  gemini-2.0-flash-exp:free              47  ✓

  » Tip: Use 'gptcode stats --today' for today's activity

────────────────────────────────────────────────────────────
```

### Mode Management

Simple switching between cloud and local execution:

```bash
gptcode mode              # Show current mode
gptcode mode cloud        # Use cloud providers (OpenRouter, Groq)
gptcode mode local        # Use Ollama only
```

## Real-World Example

**Before** (fixed profiles):
```yaml
profiles:
  router: llama-3.3-70b-versatile  # $0.59/1M, 14k daily limit
  editor: llama-3.3-70b-versatile
  reviewer: llama-3.3-70b-versatile
```

Cost: ~$5-10/month, frequent rate limits

**After** (intelligent selection):
```yaml
mode: cloud  # That's it!
```

The system automatically:
1. Tries Gemini 2.0 Flash (free, 1M context, 150 tok/s)
2. Falls back to Llama 3.2 3B (free) if rate limited
3. Uses Groq's paid models only when free tier exhausted
4. Learns from failures and avoids problematic models

Cost: ~$0-2/month, zero rate limit issues

## The Model Catalog

`~/.gptcode/models_catalog.json` defines available models:

```json
{
  "openrouter": {
    "models": [{
      "id": "google/gemini-2.0-flash-exp:free",
      "cost_per_1m": 0,
      "rate_limit_daily": 1000,
      "context_window": 1000000,
      "tokens_per_sec": 150,
      "capabilities": {
        "supports_tools": true,
        "supports_file_operations": true
      }
    }]
  }
}
```

Enrich with new models:
```bash
python3 ml/scripts/enrich_catalog.py
```

## ML-Powered Learning (Coming Soon)

The system already records feedback after each execution:
- Success: +20 score bonus
- Failure: -40 score penalty

After 20-30 tasks, train an ML classifier:
```bash
python3 ml/model_selection/train.py
```

The model learns patterns like:
- "For Go package refactoring, prefer Model A"
- "For Python simple edits, Model B works fine"
- "Model C fails on Elixir, avoid it"

## Summary

Intelligent model selection delivers:
- **10x cost reduction** via free model prioritization
- **Zero downtime** with automatic fallback
- **Better UX** with speed-optimized choices
- **Continuous learning** from historical feedback

Try it today:
```bash
gptcode mode cloud
gt do "refactor auth module"
gptcode stats
```

---

*Have questions about model selection? Join our [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)*

## See Also

- [Groq Optimal Configurations](2025-11-15-groq-optimal-configs) - Budget-friendly model setups
- [OpenRouter Multi-Provider](2025-11-16-openrouter-multi-provider) - Access to 200+ models
- [ML-Powered Intelligence](2025-11-22-ml-powered-intelligence) - ML capabilities in GPTCode
