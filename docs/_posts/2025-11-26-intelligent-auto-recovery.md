---
layout: post
title: "Intelligent Efficiency: gt do Finds The Optimal Path"
date: 2025-11-26
author: Jader Correa
description: "gt do uses intelligent auto-recovery to find the optimal model path. Evaluates cost, speed, reliability across backends with real-time learning from execution history."
tags: [features, ml, auto-recovery, intelligence, optimization]
---

# Intelligent Efficiency: `gt do` Finds The Optimal Path

Today we're releasing `gt do`—an autonomous execution system that doesn't just recover from failures, it actively **optimizes for efficiency**. The system evaluates cost, speed, reliability, and availability to find the best route to complete your task.

## The Problem

You want to complete a task. The system could use:
- An expensive premium model that works
- A fast but unreliable model
- A free model that might be slow
- A local model that's private but limited

**Traditional approach:** Pick one and hope it works.

**What if the system could evaluate all options and choose the most efficient path automatically?**

## Enter: Intelligent Efficiency

```bash
$ gt do "create a hello.txt file with Hello World" --verbose

Backend: groq
Editor Model: moonshotai/kimi-k2-instruct-0905

❌ Attempt 1 failed: tool 'read_file' not available

🤔 Evaluating all available options...

💡 Intelligence recommends: openrouter/moonshotai/kimi-k2:free
   Overall Score: 0.88
   Success Rate: 100% | Speed: 300 TPS | Cost: $0.000/1M | Latency: 20191ms
   Reason: Success: 100% (4 tasks), Speed: 300 TPS, Cost: $0.00/1M

🔄 Switching to optimal model...

=== Attempt 2/3 ===
✓ Task completed successfully
```

No user intervention. No config editing. The system:
- Detected the failure
- **Evaluated all available options** across backends
- Calculated efficiency scores considering: success rate, speed, cost, latency
- Chose the **optimal model** (free, fast, 100% success)
- Switched automatically
- Succeeded

## How It Works

### 1. Execution History

Every task execution is recorded to `~/.gptcode/task_execution_history.jsonl`:

```json
{
  "timestamp": "2025-11-24T14:30:38Z",
  "task": "create a hello.txt file",
  "backend": "groq",
  "model": "moonshotai/kimi-k2-instruct-0905",
  "success": false,
  "error": "tool 'read_file' not available"
}
```

This isn't just logging—it's a **training dataset**.

### 2. Real-Time Learning

The intelligence system calculates success rates per model/backend:

```
groq/moonshotai/kimi-k2-instruct-0905: 0% (3 failures)
openrouter/moonshotai/kimi-k2:free: 100% (4 successes)
```

### 3. Confidence-Based Recommendations

**Initial recommendation** (no history):
```
💡 Intelligence recommends: openrouter/moonshotai/kimi-k2:free
   Confidence: 50%
   Reason: Known to support function calling
```

**After learning** (≥3 tasks):
```
💡 Intelligence recommends: openrouter/moonshotai/kimi-k2:free
   Confidence: 100%
   Reason: Historical success rate: 100% (3 tasks)
```

The system **improves recommendations over time** based on your actual usage patterns.

## Multi-Criteria Optimization

The system doesn't just pick "any working model"—it finds the **most efficient** one.

### Scoring Formula

```
Score = 0.5 * SuccessRate + 0.2 * Speed + 0.2 * Cost + 0.1 * Availability
```

**Weights explained:**
- **50% Success Rate**: Reliability is most important
- **20% Speed**: Fast models = better UX
- **20% Cost**: Free models preferred when viable
- **10% Availability**: Rate limits matter

### Example Calculation

**openrouter/kimi:free**:
- Success: 100% = 0.50
- Speed: 300 TPS = 0.06 (300/1000 * 0.2)
- Cost: $0/1M = 0.20 (free = max score)
- Availability: 100% = 0.10
- **Total: 0.86**

**groq/llama-70b**:
- Success: 0% = 0.00
- Speed: 500 TPS = 0.10
- Cost: $0/1M = 0.20
- Availability: 100% = 0.10  
- **Total: 0.40**

→ System chooses openrouter/kimi:free (higher score)

## Not Just Fallback Logic

This isn't a hardcoded list of "if model X fails, try model Y."

Traditional fallback:
```go
if err := tryModel("groq/model-a"); err != nil {
    return tryModel("openrouter/model-b")  // Hardcoded
}
```

Intelligence-based:
```go
if err := tryModel(currentModel); err != nil {
    // Query ML system
    history := getExecutionHistory(limit=100)
    recommendations := calculateSuccessRates(history, taskType)
    bestModel := recommendations.sortByConfidence()[0]
    return tryModel(bestModel)  // Data-driven
}
```

Key differences:
- **Adapts to your setup** (not universal defaults)
- **Learns from failures** (improves over time)
- **Cross-backend switching** (not limited to one provider)
- **Confidence scores** (transparency in decision-making)

## Backend Switching

The system can automatically switch backends during retry:

```
Attempt 1: groq/model-x → Failed
Attempt 2: openrouter/model-y → Success ✓
```

This requires both backends to be configured, but the intelligence system will discover which combination works best **for your specific use case**.

## Real-World Example

Let's trace a real execution:

### First Time (Cold Start)

```bash
$ gt do "create config.yaml" --verbose

# Attempt 1 with default model
Backend: groq
Model: moonshotai/kimi-k2-instruct-0905
❌ Failed: tool not available

# Intelligence recommendation (no history yet)
💡 Recommends: openrouter/moonshotai/kimi-k2:free
   Confidence: 50%
   Reason: Known to support function calling

# Retry succeeds
✓ Task completed
```

**System learned**: `openrouter/kimi:free` works for this task type.

### Second Time

```bash
$ gt do "create database.yaml" --verbose

# Still tries default first (respects user config)
❌ Failed: tool not available

# Now has 1 success in history
💡 Recommends: openrouter/moonshotai/kimi-k2:free
   Confidence: 50%  # Still < 3 tasks
   
✓ Task completed
```

**System learned**: Second success with `openrouter/kimi:free`.

### Third Time

```bash
$ gt do "create api.yaml" --verbose

❌ Failed: tool not available

# Now has 2 successes in history
💡 Recommends: openrouter/moonshotai/kimi-k2:free
   Confidence: 50%  # Still < 3 tasks
   
✓ Task completed
```

### Fourth Time (Confidence Kicks In)

```bash
$ gt do "create server.yaml" --verbose

❌ Failed: tool not available

# Now has ≥3 tasks: uses historical success rate
💡 Recommends: openrouter/moonshotai/kimi-k2:free
   Confidence: 100%  # 3/3 successes!
   Reason: Historical success rate: 100% (3 tasks)
   
✓ Task completed
```

**System is now confident**: `openrouter/kimi:free` is the right choice for this user's setup.

## Why This Matters

### 1. Zero Configuration After Setup

Once you've configured multiple backends, the system figures out the optimal combination for you.

### 2. Adapts to Your Environment

Different users have different:
- API quotas
- Model availability
- Network conditions
- Cost constraints

The intelligence system learns **your specific patterns**, not universal defaults.

### 3. Improves With Usage

The more you use `gt do`, the smarter it gets. No manual tuning required.

### 4. Transparent Decision-Making

Every recommendation comes with:
- Confidence score
- Reasoning
- Historical data

You always know **why** the system chose a particular model.

## Command Usage

### Basic
```bash
gt do "create a file"
```

### With Verbose (Recommended Initially)
```bash
gt do "create a file" --verbose
```

Shows:
- Which models are being tried
- Why alternatives are recommended
- Confidence scores
- Success/failure details

### Dry Run (Analysis Only)
```bash
gt do "complex refactoring" --dry-run
```

Analyzes the task without executing.

### Control Retries
```bash
gt do "task" --max-attempts 5
```

Default is 3 attempts.

## Viewing Your History

```bash
# Raw history
cat ~/.gptcode/task_execution_history.jsonl

# Analyze success rates
cat ~/.gptcode/task_execution_history.jsonl | \
  jq -s 'group_by(.backend + "/" + .model) | 
         map({
           model: (.[0].backend + "/" + .[0].model),
           success_rate: (map(select(.success)) | length) / length,
           total: length
         })'
```

Example output:
```json
[
  {
    "model": "groq/moonshotai/kimi-k2-instruct-0905",
    "success_rate": 0,
    "total": 4
  },
  {
    "model": "openrouter/moonshotai/kimi-k2:free",
    "success_rate": 1,
    "total": 4
  }
]
```

Clear pattern: Switch to OpenRouter for this user's setup.

## Technical Implementation

### Intelligence Package

New `internal/intelligence/` package with:

**history.go**
- `RecordExecution()` - Persist task results
- `GetRecentModelPerformance()` - Calculate success rates
- JSONL format for easy analysis

**recommender.go**
- `RecommendModelForRetry()` - ML-based model selection
- Considers: history, capabilities, backend availability
- Returns sorted recommendations with confidence scores

### Auto-Recovery Flow

```go
func runDoExecutionWithRetry(task string, maxAttempts int) error {
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        err := runDoExecution(task, currentModel)
        
        // Record result
        intelligence.RecordExecution(TaskExecution{
            Task: task,
            Model: currentModel,
            Success: err == nil,
            Error: err.Error(),
        })
        
        if err == nil {
            return nil  // Success!
        }
        
        if !isToolError(err) {
            return err  // Different type of error
        }
        
        // Get recommendation
        recs, _ := intelligence.RecommendModelForRetry(
            setup, "editor", currentBackend, currentModel, task
        )
        
        // Retry with recommended model
        currentModel = recs[0].Model
        currentBackend = recs[0].Backend
    }
    
    return fmt.Errorf("failed after %d attempts", maxAttempts)
}
```

### Guided Mode Extension

Added `NewGuidedModeWithCustomModel()` to allow model override during retry:

```go
type GuidedMode struct {
    model       string  // Query model
    editorModel string  // Can be different during retry
}
```

This enables switching the editor model while keeping the same orchestrator/provider.

## Future Enhancements

Current version uses simple success rate calculation. Planned improvements:

### 1. Task Feature Extraction
- Complexity estimation
- File count
- Language detection
- Operation type (read vs write)

### 2. Cost Optimization
- Factor in model pricing
- Prefer cheaper models when confidence is similar

### 3. Latency Awareness
- Track execution time
- Prefer faster models for simple tasks

### 4. Advanced ML Models
- XGBoost ensemble
- KAN (Kolmogorov-Arnold Networks)[^1][^2]
- Multi-objective optimization

See [Intelligence Layers notebook](../notebooks/intelligence-layers.md) for the full ML roadmap.

---

## References

[^1]: Liu, Z., Wang, Y., Vaidya, S., Ruehle, F., Halverson, J., Soljačić, M., Hou, T. Y., & Tegmark, M. (2024). KAN: Kolmogorov-Arnold Networks. arXiv:2404.19756 [cs.LG]. [https://arxiv.org/abs/2404.19756](https://arxiv.org/abs/2404.19756)

[^2]: Liu, Z., Ma, P., Wang, Y., Matusik, W., & Tegmark, M. (2024). KAN 2.0: Kolmogorov-Arnold Networks Meet Science. arXiv:2408.10205 [cs.LG]. [https://arxiv.org/abs/2408.10205](https://arxiv.org/abs/2408.10205)

## Comparison: gt do vs gptcode guided

| Feature | gt do | gptcode guided |
|---------|--------|------------|
| User approval | None | Required |
| Auto-recovery | ✓ With learning | ✗ Manual fix |
| Learning | ✓ Improves over time | ✗ Static |
| Speed | Fast (automatic retry) | Slower (human review) |
| Safety | Medium | High |
| Best for | Quick tasks, iteration | High-risk changes |

## Getting Started

### 1. Update GPTCode
```bash
cd ~/gptcode
git pull origin main
go build -o bin/gptcode cmd/gptcode/*.go
```

### 2. Configure Multiple Backends
```bash
gt setup
# Add at least 2 backends (e.g., groq + openrouter)
```

### 3. Try It Out
```bash
gt do "create a test.txt file with Hello" --verbose
```

### 4. Watch It Learn
Run a few more tasks and observe confidence scores increasing.

### 5. Check Your Stats
```bash
cat ~/.gptcode/task_execution_history.jsonl | jq
```

## Best Practices

### Let It Learn
Don't intervene manually during retries. The system needs real failure/success data to learn.

### Use Verbose Mode Initially
```bash
gt do "task" --verbose
```

Helps you understand:
- Which models work in your setup
- Why certain recommendations are made
- How confidence builds over time

### Configure Diverse Backends
More backends = more alternatives:
- **Groq**: Fast, cheap (some models lack tools)
- **OpenRouter**: Many free options with tools
- **Ollama**: Local, private
- **OpenAI**: Premium, reliable

### Don't Reset History
`task_execution_history.jsonl` is your trained model. Preserve it across reinstalls.

## Known Limitations

### Cold Start Problem
First few executions have lower confidence (50%). After ≥3 tasks, confidence becomes data-driven.

### Requires Multiple Backends
If you only have one backend configured, the system can't switch. Configure at least 2.

### Tool-Error Specific
Currently only triggers on function calling errors. Other failure modes may not auto-recover.

## Community Feedback

We'd love to hear:
- How well does the system learn in your setup?
- Which model combinations work best?
- What confidence threshold feels right for auto-retry?

Open an issue or discussion on [GitHub](https://github.com/jadercorrea/gptcode).

---

*Posted on November 26, 2025. Tested on commit b462c9f with real execution data.*
