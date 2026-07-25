---
layout: post
title: "Preparing for Autonomous Execution: File Validation, Telemetry & Intelligence"
date: 2025-11-27
author: Jader Correa
description: "Foundation for autonomous execution: file validation prevents scope creep, explicit success criteria enable verification, telemetry tracks execution, model catalog centralizes capabilities."
tags: [features, architecture, autonomous, validation, telemetry]
---

# Preparing for Autonomous Execution: File Validation, Telemetry & Intelligence

Today we're announcing a set of foundational improvements that bring gptcode closer to **autonomous execution**—the ability to run implementation tasks automatically when repository issues are created, with built-in validation, telemetry, and intelligent model selection.

## The Vision

Imagine opening a GitHub issue with:

```markdown
Title: Add user authentication
Description: Implement JWT-based authentication with refresh tokens
```

And having gptcode:
1. Automatically detect the new issue
2. Create an implementation plan
3. Execute the plan with file validation
4. Verify the changes work
5. Track telemetry for continuous improvement
6. Open a PR with the implementation

**This release lays the groundwork for that future.**

## What's New

### 1. File Validation & Tracking

**The Problem:** In autonomous execution, the agent could modify files outside the scope of the plan, causing unintended side effects.

**The Solution:** Both `write_file` and `apply_patch` now:
- Enforce allowlist validation (only modify planned files)
- Return the actual files modified
- Pass real modifications to the Validator (not just the plan's whitelist)

```go
// Before: No validation on apply_patch
result := tools.ApplyPatch(call, workdir)

// After: Validated and tracked
result := tools.ApplyPatch(call, workdir)
// result.ModifiedFiles = ["internal/auth/handler.go"]
```

**Why it matters:**
- Prevents scope creep in autonomous execution
- Validator sees actual changes, not assumptions
- Better error messages when validation fails

---

### 2. Explicit Success Criteria in Plans

**The Problem:** Plans had vague validation steps like "verify it worked."

**The Solution:** The Planner now requires **2-5 specific, testable success criteria**:

```markdown
## Success Criteria
- Tests pass: make test
- File internal/auth/handler.go contains JWT validation
- Command curl /api/auth/login returns 401 without token
- Documentation updated in docs/auth.md
```

**Why it matters:**
- Validator can check concrete conditions
- Plans are more actionable
- Autonomous execution knows when to stop

---

### 3. Model Capability Catalog

**The Problem:** Model capabilities (tool-calling, cost, speed) were hardcoded in multiple places.

**The Solution:** Created `ModelCatalog` to centralize model metadata:

```go
catalog := intelligence.NewModelCatalog()
models := catalog.GetModelsForAgent("editor")

for _, model := range models {
    fmt.Printf("%s: $%.2f/1M, %d TPS, Functions: %v\n",
        model.Name, model.CostPer1M, model.SpeedTPS, model.SupportsFunctions)
}
```

**Fallback support:**
```go
// Unknown model? Returns sensible defaults
info := catalog.GetModelInfo("new-backend", "new-model")
// info.CostPer1M = 1.0 (default)
// info.SpeedTPS = 300 (default)
```

**Why it matters:**
- Easy to add new models (update catalog only)
- Consistent capabilities across the system
- Graceful handling of unknown models

---

### 4. OpenTelemetry-Based Telemetry

**The Problem:** No visibility into what the system does during autonomous execution.

**The Solution:** Implemented OpenTelemetry-based telemetry with:

**Step tracking:**
```go
tel := telemetry.NewTelemetry()
event := telemetry.StepEvent{
    StepIndex:    0,
    StepName:     "Implement Authentication",
    FilesTouched: []string{"auth/handler.go", "auth/middleware.go"},
    Success:      true,
    DurationMs:   2500,
}
tel.RecordStep(ctx, event)
```

**Usage tracking:**
```go
tracker := telemetry.NewUsageTracker()
tracker.RecordRequest("openai", "gpt-4", 1500)

stats := tracker.GetStats()
// stats["openai/gpt-4"] = {Requests: 1, Tokens: 1500}
```

**Why it matters:**
- Observe autonomous execution in real-time
- Track API costs per backend/model
- Debug failures with structured events
- Foundation for `gptcode usage` command

---

## Technical Deep Dive

### File Validation Flow

```go
// agent/editor.go
func (e *EditorAgent) Execute(ctx context.Context, history []llm.ChatMessage, 
    statusCallback StatusCallback) (string, []string, error) {
    
    var modifiedFiles []string
    
    for _, toolCall := range resp.ToolCalls {
        // Validate write_file AND apply_patch
        if toolCall.Name == "write_file" || toolCall.Name == "apply_patch" {
            if err := e.validateFileWrite(argsMap); err != nil {
                // Reject modification outside allowlist
                return "", nil, err
            }
        }
        
        result := tools.ExecuteToolFromLLM(toolCall, e.cwd)
        modifiedFiles = append(modifiedFiles, result.ModifiedFiles...)
    }
    
    return response, modifiedFiles, nil
}
```

### Model Catalog Structure

```go
type ModelCatalog struct {
    Models map[string]ModelInfo
}

type ModelInfo struct {
    Backend           string
    Name              string
    SupportsFunctions bool
    CostPer1M         float64
    SpeedTPS          int
    Agents            []string // ["editor", "query", ...]
}

// Usage
catalog := NewModelCatalog()
editorModels := catalog.GetModelsForAgent("editor")
```

### Telemetry Integration Points

**In Guided Mode:**
```go
func (g *GuidedMode) Implement(ctx context.Context, plan string) error {
    tel := telemetry.NewTelemetry()
    start := time.Now()
    
    result, modifiedFiles, err := editorAgent.Execute(ctx, history, statusCallback)
    
    tel.RecordStep(ctx, telemetry.StepEvent{
        StepName:     "Implementation",
        FilesTouched: modifiedFiles,
        Success:      err == nil,
        DurationMs:   time.Since(start).Milliseconds(),
    })
    
    return err
}
```

**In Orchestrated Mode:**
```go
func (m *Maestro) executeStep(ctx context.Context, step PlanStep) error {
    tel := telemetry.NewTelemetry()
    
    _, modifiedFiles, err := editorAgent.Execute(ctx, history, statusCallback)
    
    tel.RecordStep(ctx, telemetry.StepEvent{
        StepIndex:    m.CurrentStepIdx,
        StepName:     step.Title,
        FilesTouched: modifiedFiles,
        Success:      err == nil,
    })
    
    return err
}
```

---

## Testing

All improvements come with comprehensive unit tests:

**Catalog tests:**
```bash
$ go test -v ./internal/intelligence/...
=== RUN   TestNewModelCatalog
--- PASS: TestNewModelCatalog
=== RUN   TestGetModelsForAgent
--- PASS: TestGetModelsForAgent
=== RUN   TestGetModelInfo
--- PASS: TestGetModelInfo
PASS
```

**Telemetry tests:**
```bash
$ go test -v ./internal/telemetry/...
=== RUN   TestRecordStep
--- PASS: TestRecordStep
=== RUN   TestUsageTrackerMultipleModels
--- PASS: TestUsageTrackerMultipleModels
PASS
```

---

## Impact on Existing Workflows

### Guided Mode (`gptcode guided`)

**Before:**
```bash
gptcode guided "add auth"
# Validator checks plan whitelist (not actual changes)
```

**After:**
```bash
gptcode guided "add auth"
# Validator checks actually modified files
# Telemetry records what was changed
```

### Orchestrated Mode (Maestro)

**Before:**
```bash
gptcode auto plan.md
# No file validation on apply_patch
# No telemetry on steps
```

**After:**
```bash
gptcode auto plan.md
# Both write_file and apply_patch validated
# Every step emits telemetry events
```

---

## Roadmap to Autonomous Execution

**Note:** The following features are part of the roadmap and not yet implemented. The improvements described above lay the foundation for these capabilities.

Still needed:

### 1. GitHub Actions Integration
Trigger execution on issue creation:
```yaml
on:
  issues:
    types: [opened]
    
jobs:
  gptcode-auto:
    runs-on: ubuntu-latest
    steps:
      - run: gptcode auto --from-issue ${{ github.event.issue.number }}
```

### 2. Usage Command (Future)
```bash
$ gptcode usage
Usage Statistics
================

openai/gpt-4:
  Requests: 15
  Tokens:   45000
  Cost:     $0.45

openrouter/kimi:free:
  Requests: 32
  Tokens:   120000
  Cost:     $0.00

Total:
  Requests: 47
  Tokens:   165000
  Cost:     $0.45
```

### 3. Graph Context Injection (Future)
Language-specific heuristics to inject relevant code:
- Extract imports/signatures in Go
- Include only referenced functions
- Limit tokens per file

### 4. End-to-End Verification (Future)
```bash
gptcode verify plan.md
# Runs all success criteria
# Returns structured results
```

---

## What You Can Do Today

### 1. Try File Validation

Create a plan with specific files:
```bash
gt plan "add logout endpoint"
gptcode implement ~/.gptcode/plans/2025-11-27-logout.md
```

The editor will only modify files mentioned in the plan.

### 2. Check Telemetry

```bash
export GPTCODE_DEBUG=1
gptcode guided "add feature"
```

Look for step events in stderr.

### 3. Explore the Catalog

```go
import "gptcode/internal/intelligence"

catalog := intelligence.NewModelCatalog()
for key, info := range catalog.Models {
    fmt.Printf("%s: $%.2f/1M\n", key, info.CostPer1M)
}
```

### 4. Run Tests

```bash
go test ./internal/intelligence ./internal/telemetry
```

---

## Community Feedback

We're building autonomous execution **with** the community. We'd love to hear:

- Which features are most important for autonomous execution?
- Should we prioritize GitHub integration or local improvements first?
- What telemetry data would be most valuable?

Join the discussion on [GitHub](https://github.com/jadercorrea/gptcode/issues).

---

## References

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/languages/go/)
- [Intelligent Auto Recovery](./2025-11-26-intelligent-auto-recovery.html)
- [Complete Workflow Guide](./2025-11-24-complete-workflow-guide.html)

---

*Posted on November 27, 2025. All features tested with unit tests and integration workflows.*
