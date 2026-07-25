---
layout: post
title: "Dependency Graph: 5x Token Reduction Through Smart Context"
date: 2025-12-03
author: Jader Correa
description: "Using dependency graphs and PageRank to achieve 5x token reduction while improving response quality. Smart context selection for AI coding assistants."
tags: [features, optimization, context-management, performance]
---

# Dependency Graph: 5x Token Reduction Through Smart Context

In [Context Engineering]({% post_url 2025-11-14-context-engineering-for-real-codebases %}), we discussed managing context windows. Today, we're diving into **how** GPTCode achieves 5x token reduction through dependency graph analysis.

## The Problem: Context Overload

When you ask an AI coding assistant a question, it needs context. But how much?

**Naive approach**: Send the entire codebase
- 100,000+ tokens
- Expensive ($0.30-$3.00 per query with large models)
- Slow (processing time scales with context)
- Noisy (LLM sees irrelevant code)

**Traditional approach**: Keyword matching
- Search for files containing query terms
- Still sends too much or too little
- Misses important dependencies

## The Solution: Dependency Graph + PageRank

GPTCode analyzes your codebase structure to provide **only relevant context** to the LLM.

### How It Works

```
1. Build dependency graph from imports/requires
   go: import statements
   python: import/from statements
   js/ts: import/require statements
   ruby: require statements
   rust: use statements

2. Run PageRank to identify central files
   Files with many dependencies = higher importance
   Example: config.go, types.go, utils.go

3. Match query terms to relevant files
   "authentication" → auth.go, middleware.go, session.go

4. Expand to 1-hop neighbors
   Include direct dependencies and dependents
   auth.go → user.go, token.go, db.go

5. Select top N most relevant files (default: 5)
   Ranked by PageRank score + query relevance
```

### Example: Authentication Bug

**Your query**: `gt chat "fix bug in authentication"`

**Without graph** (naive):
- Sends: All 142 files (100,000 tokens)
- Cost: $0.30 with GPT-4 Turbo
- Time: 8 seconds to process

**With graph** (smart):
- Analyzes: Graph of 142 nodes, 287 edges
- Selects: 5 files (18,000 tokens)
  1. `internal/auth/handler.go` (score: 0.842)
  2. `internal/auth/middleware.go` (score: 0.731)
  3. `internal/models/user.go` (score: 0.689)
  4. `internal/auth/session.go` (score: 0.654)
  5. `config/security.go` (score: 0.612)
- Cost: $0.05 (83% savings)
- Time: 1.5 seconds (81% faster)

**Result**: Better answer, lower cost, faster response.

## Real-World Impact

### Token Reduction

Typical codebase (50-200 files):
- **Before**: 80,000-120,000 tokens per query
- **After**: 15,000-25,000 tokens per query
- **Reduction**: 5-6x fewer tokens

### Cost Savings

For 1,000 queries per month:

**Without graph**:
- 100M tokens × $0.30/1M = **$30/month** (GPT-4 Turbo input)

**With graph**:
- 20M tokens × $0.30/1M = **$6/month** (GPT-4 Turbo input)

**Savings**: $24/month (80% reduction)

With free models (OpenRouter), both costs → $0, but graph still provides **better responses** through focused context.

## Better Responses

More tokens ≠ better answers.

**The problem with too much context**:
- LLM sees noise and signal equally
- Attention diluted across irrelevant code
- Higher chance of hallucination

**Smart context wins**:
- LLM focuses on relevant code only
- Better reasoning with less distraction
- More accurate suggestions

### Example Comparison

Query: "Add rate limiting to login endpoint"

**Without graph** (entire codebase):
```
I see you have multiple authentication methods across
different files. I recommend...
[suggests changes to OAuth, SAML, JWT handlers]
[suggests modifying test files]
```
❌ Unfocused, touches too many files

**With graph** (auth + middleware only):
```
Looking at your auth/handler.go and middleware stack,
here's a focused approach:

1. Add rate limiter middleware before auth
2. Configure in security.go
3. Update handler to return 429 on limit exceeded
```
✓ Focused, actionable, correct scope

## Language Support

Currently supported:
- **Go**: `import` statements
- **Python**: `import`, `from ... import`
- **JavaScript/TypeScript**: `import`, `require`
- **Ruby**: `require`, `require_relative`
- **Rust**: `use`, `mod`

More languages coming soon (Java, C++, etc.)

## How to Use It

### Automatic (Default)

Graph analysis works transparently in `gt chat`:

```bash
gt chat "explain how routing works"
```

The system:
1. Builds graph (cached, ~100ms first time)
2. Finds relevant files
3. Sends focused context to LLM
4. Returns answer

### Debug Mode

See what the graph is doing:

```bash
GPTCODE_DEBUG=1 gt chat "your query"
```

Output:
```
[GRAPH] Building dependency graph...
[GRAPH] Built graph: 142 nodes, 287 edges
[GRAPH] Selected 5 files:
[GRAPH]   1. internal/agents/router.go (score: 0.842)
[GRAPH]   2. internal/llm/provider.go (score: 0.731)
[GRAPH]   3. cmd/gptcode/main.go (score: 0.689)
[GRAPH]   4. internal/modes/chat.go (score: 0.654)
[GRAPH]   5. internal/config/setup.go (score: 0.612)
```

### Configuration

Control how many files to include:

```bash
# View current setting
gt config get defaults.graph_max_files

# Change to 3 files (more focused)
gt config set defaults.graph_max_files 3

# Change to 10 files (broader context)
gt config set defaults.graph_max_files 10
```

**Recommendation**: Start with 5 (default), adjust based on codebase size.

### Disable (if needed)

```bash
GPTCODE_GRAPH=false gt chat "query"
```

## Technical Details

### Graph Building

The system uses Go's AST parser to extract imports:

```go
func parseGoImports(filePath string) []string {
    fset := token.NewFileSet()
    node, _ := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
    
    var imports []string
    for _, imp := range node.Imports {
        importPath := strings.Trim(imp.Path.Value, "\"")
        imports = append(imports, importPath)
    }
    return imports
}
```

Similar parsers for Python, JS, Ruby, Rust.

### PageRank Algorithm

Classic PageRank with damping factor 0.85:

```
PR(A) = (1-d)/N + d * Σ(PR(Ti)/C(Ti))

Where:
- d = 0.85 (damping factor)
- N = total nodes
- Ti = nodes linking to A
- C(Ti) = outbound links from Ti
```

Runs for 20 iterations, converges in ~50ms for typical codebases.

### Query Matching

Simple term frequency:

```go
func scoreFileForQuery(file string, queryTerms []string) float64 {
    content := readFile(file)
    score := 0.0
    for _, term := range queryTerms {
        score += float64(strings.Count(content, term))
    }
    return score
}
```

Combined with PageRank to get final relevance score.

### Caching

Graph is cached in memory and rebuilt only when:
- Files added/removed
- Imports modified
- 5 minutes elapsed (stale check)

## Performance

Benchmarks on medium codebase (150 files):

```
Graph build:          98ms (first time)
Graph load (cached):  2ms
File selection:       5ms
Total overhead:       7ms per query (cached)
```

The 5x token reduction more than compensates for the 7ms overhead.

## Future Enhancements

### 1. Semantic Search Integration

Combine graph analysis with embeddings:
- Graph: structural relevance
- Embeddings: semantic similarity
- Hybrid score for best of both

### 2. Function-Level Granularity

Currently file-level. Planned:
- Extract function definitions
- Build function-level dependency graph
- Include only relevant functions, not entire files

### 3. Change Impact Analysis

Predict which files might break:
```bash
gptcode impact "change auth signature"
# Shows: Files that import auth + test files
```

### 4. Multi-Language Projects

Better handling of polyglot codebases:
- Go backend + TypeScript frontend
- Python ML + Go API
- Cross-language dependency tracking

## Comparison: Graph vs Embeddings

| Aspect | Dependency Graph | Semantic Embeddings |
|--------|------------------|---------------------|
| **Basis** | Code structure | Content meaning |
| **Speed** | Fast (~5ms) | Slower (~100ms) |
| **Accuracy** | High for refactoring | High for exploration |
| **Setup** | Zero | Requires model |
| **Cost** | Free | Free (local models) |
| **Best for** | "fix bug in X" | "how does X work?" |

**GPTCode's approach**: Start with graph (fast, structural), add embeddings later for semantic queries.

## Getting Started

### 1. Update GPTCode

```bash
cd ~/gptcode
git pull origin main
make install
```

### 2. Try It Out

```bash
# Enable debug to see graph in action
GPTCODE_DEBUG=1 gt chat "explain authentication flow"
```

### 3. Observe the Magic

Watch as GPTCode:
- Builds the graph (first time only)
- Selects 5 relevant files
- Provides focused, accurate answer

### 4. Compare

Try the same query with graph disabled:
```bash
GPTCODE_GRAPH=false gt chat "explain authentication flow"
```

Notice:
- Slower response
- Less focused answer
- Higher token usage (check with `gptcode stats`)

## Best Practices

### Let the Cache Work

Don't disable graph unless debugging. The cache makes it nearly free after first build.

### Use Descriptive Queries

Better queries → better file selection:
- ✓ "fix bug in authentication middleware"
- ✗ "fix bug"

### Check Debug Output

If answers seem off, use `GPTCODE_DEBUG=1` to see which files were selected. Adjust `graph_max_files` if needed.

### Combine with Other Features

Graph works great with:
- **ML intent classification**: Fast routing to right agent
- **Model selection**: Optimal model for each query
- **Feedback system**: Learn from corrections

## Community Examples

Real user queries where graph made a difference:

**Query**: "Add logging to database operations"
- **Selected**: db.go, logger.go, config.go (3 files, 8k tokens)
- **Result**: Precise changes, 2 minutes to implement

**Query**: "Refactor error handling"
- **Selected**: errors.go, handler.go, middleware.go, utils.go, types.go (5 files, 15k tokens)
- **Result**: Comprehensive refactoring plan, consistent across codebase

**Query**: "Why is the API slow?"
- **Selected**: handler.go, db.go, cache.go, middleware.go (4 files, 12k tokens)
- **Result**: Identified N+1 query issue, suggested caching

## Summary

Dependency graph delivers:
- **5x token reduction** (100k → 20k tokens)
- **80% cost savings** on cloud models
- **Better responses** through focused context
- **Faster processing** (less context = quicker)
- **Automatic** (zero configuration)

Try it today:
```bash
gt chat "your question about the codebase"
```

---

## References

- Page, L., Brin, S., Motwani, R., & Winograd, T. (1999). The PageRank Citation Ranking: Bringing Order to the Web. *Stanford InfoLab*.

---

*Have questions about dependency graphs? Join our [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)*

## See Also

- [Context Engineering](2025-11-14-context-engineering-for-real-codebases) - Making AI work in real codebases
- [Agent Routing vs Tool Search](2025-12-01-agent-routing-vs-tool-search) - Context reduction strategies
- [Intelligent Model Selection](2025-12-02-intelligent-model-selection) - Cost optimization
