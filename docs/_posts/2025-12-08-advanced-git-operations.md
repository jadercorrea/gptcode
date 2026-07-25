---
layout: post
title: "Advanced Git Operations: Achieving 100% Autonomy with AI-Powered Git"
date: 2025-12-08
author: Jader Correa
description: "GPTCode now handles advanced Git operations autonomously: bisect for bug hunting, cherry-pick and rebase with conflict resolution, squash with AI-generated messages, reword for commit improvements, and intelligent merge conflict resolution."
tags: [features, git, autonomy, milestone, conflict-resolution]
---

# Advanced Git Operations: Achieving 100% Autonomy with AI-Powered Git

Today marks a significant milestone: **gptcode achieves 100% autonomy (64/64 scenarios)** by mastering the most challenging Git operations that previously required human intervention.

## The Challenge: Git's Complex Operations

While basic Git commands (commit, push, pull) are straightforward, real-world development involves complex scenarios:

1. **Bug hunting** - Finding which commit introduced a regression
2. **Conflict resolution** - Merging divergent code changes
3. **History management** - Squashing, rewording, cherry-picking commits
4. **Branch operations** - Rebasing with conflicts
5. **Database migrations** - Schema evolution without downtime

These operations traditionally require:
- Deep Git knowledge
- Understanding of codebase context
- Human judgment for conflict resolution
- Careful commit message crafting

**Until now.**

## What's New: 6 Advanced Git Commands

### 1. Git Bisect: Automated Bug Hunting

**The Problem:** You know a test is failing, but which of the last 50 commits broke it?

**The Solution:** AI-powered binary search through Git history:

```bash
gptcode git bisect --bad HEAD --good v1.2.0 --test "go test ./internal/auth"
```

How it works:
1. Runs your test command at each commit
2. Uses binary search to find the breaking commit
3. LLM analyzes the diff to explain what went wrong
4. Returns exact commit hash and AI explanation

**Real-world example:**
```bash
$ gptcode git bisect --bad HEAD --good abc123 --test "npm test"

🔍 Starting bisect: 24 commits to check (~4 steps)
  Step 1/4: Testing commit def456... ✓ GOOD
  Step 2/4: Testing commit ghi789... ✗ BAD
  Step 3/4: Testing commit jkl012... ✗ BAD
  Step 4/4: Testing commit mno345... ✓ GOOD

🎯 Found breaking commit: jkl012

📊 AI Analysis:
The bug was introduced in commit jkl012 where the authentication
middleware was refactored. The change removed token validation
from the early return path, allowing unauthenticated requests
to proceed. Fix: restore validation check before handler execution.

Changed files:
- internal/auth/middleware.go (line 45-67)
```

### 2. Cherry-Pick with AI Conflict Resolution

**The Problem:** You want to apply a commit from another branch, but conflicts arise.

**The Solution:** Intelligent conflict resolution that understands your code:

```bash
gptcode git cherry-pick abc123
```

When conflicts occur:
- AI analyzes both versions
- Understands the intent of each change
- Generates resolved code that preserves both intentions
- Explains the resolution strategy

**Example conflict resolution:**
```bash
$ gptcode git cherry-pick feature/new-auth

⚠️  Conflict in internal/auth/handler.go

🤖 AI Resolution:
Both changes modify the authentication flow. The cherry-picked
commit adds rate limiting, while current branch adds 2FA.
Merged both features by:
1. Keeping 2FA validation first (security priority)
2. Adding rate limiting after auth success
3. Preserving error handling from both versions

✓ Conflict resolved automatically
```

### 3. Rebase with AI Conflict Resolution

**The Problem:** Rebasing feature branches on main often causes conflicts that require expert judgment.

**The Solution:** Autonomous rebase that handles conflicts intelligently:

```bash
gptcode git rebase main
```

Features:
- Detects and resolves conflicts automatically
- Maintains semantic correctness
- Preserves intent of both branches
- Falls back to manual resolution for ambiguous cases

**Example:**
```bash
$ gptcode git rebase main

🔄 Rebasing 5 commits from feature/api onto main...
  Commit 1/5: Add user endpoint... ✓
  Commit 2/5: Update middleware... ⚠️  Conflict detected
  
  🤖 Analyzing conflict in internal/server/router.go...
  
  Resolution: Both branches added middleware, but in different order.
  New order based on dependency analysis:
  1. AuthMiddleware (required first)
  2. RateLimitMiddleware (added by main)
  3. LoggingMiddleware (added by feature)
  
  ✓ Resolved and continuing...
  Commit 3/5: Add tests... ✓
  Commit 4/5: Update docs... ✓
  Commit 5/5: Fix linting... ✓

✨ Rebase completed successfully
```

### 4. Squash Commits with AI-Generated Messages

**The Problem:** Feature branches accumulate many small commits that need squashing with a meaningful message.

**The Solution:** AI that understands your changes and writes professional commit messages:

```bash
gptcode git squash --count 5
# or
gptcode git squash --from abc123
```

The AI analyzes all commits and generates a structured message with:
- Clear subject line following conventional commits
- Detailed body explaining the complete change
- Context about why the change was made

**Example:**
```bash
$ gptcode git squash --count 3

📝 Analyzing 3 commits:
  - fix: handle nil pointer in auth
  - wip: update tests
  - typo: fix comment

🤖 Generated commit message:

────────────────────────────────────────
fix: Improve authentication error handling

Refactored authentication middleware to properly handle edge cases:
- Add nil pointer checks before token validation
- Update test suite to cover nil token scenarios
- Improve code documentation clarity

This prevents crashes when malformed auth headers are received
and makes the authentication flow more robust.
────────────────────────────────────────

✓ 3 commits squashed into 1
```

### 5. Reword Commits with AI Suggestions

**The Problem:** Commit messages like "wip", "fix", or "update stuff" don't help future maintainers.

**The Solution:** AI that reads your diff and suggests professional messages:

```bash
gptcode git reword abc123
```

Features:
- Analyzes the actual code changes
- Follows conventional commit format
- Provides context about why the change was made
- Offers multiple suggestions to choose from

**Example:**
```bash
$ gptcode git reword HEAD

Current message: "update handler"

🤖 AI Suggestions:

1. refactor(auth): Simplify authentication handler logic
   
   Replace nested if statements with early returns for better
   readability. No functional changes.

2. feat(auth): Add request timeout to authentication handler
   
   Implement 30-second timeout for auth requests to prevent
   hanging connections. Returns 408 Request Timeout on expiry.

3. fix(auth): Correct error handling in authentication flow
   
   Fix bug where authentication errors were not properly logged.
   Now all auth failures include request ID for debugging.

Which message? (1-3, or edit): 
```

### 6. Merge Conflict Resolution

**The Problem:** Merge conflicts require manual resolution and deep understanding of both branches.

**The Solution:** Standalone conflict resolver that can be called anytime:

```bash
gptcode git resolve-conflicts
```

Works with any Git operation that causes conflicts:
- `git merge`
- `git pull`
- `git cherry-pick`
- `git rebase`

**Example:**
```bash
$ git merge feature/new-api
# ... conflicts occur ...

$ gptcode git resolve-conflicts

🔍 Found 3 conflicted files:
  - internal/api/handler.go
  - internal/api/routes.go
  - internal/api/types.go

📝 Resolving internal/api/handler.go...

🤖 Analysis:
Both branches modified the CreateUser handler:
- Current branch: Added validation for email format
- Incoming branch: Added validation for password strength

Resolution strategy: Combine both validations in sequence.

✓ Resolved internal/api/handler.go

📝 Resolving internal/api/routes.go...
✓ Resolved internal/api/routes.go

📝 Resolving internal/api/types.go...
✓ Resolved internal/api/types.go

✨ All conflicts resolved!
Run 'git add .' to stage the resolutions.
```

## Zero-Downtime Schema Evolution

Bonus feature for database operations: intelligent migration strategies.

```bash
gptcode gen migration --zero-downtime
```

For operations like "add NOT NULL column to users table", the AI generates a multi-phase migration:

```sql
-- Phase 1: Add column as nullable
ALTER TABLE users ADD COLUMN email VARCHAR(255);

-- Phase 2: Backfill data (deploy safely)
UPDATE users SET email = legacy_email WHERE email IS NULL;

-- Phase 3: Add NOT NULL constraint
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
```

## Configuration

All Git operations respect your LLM provider settings:

```yaml
# ~/.gptcode/config.yaml
mode: cloud  # or local

# Git operations use the same intelligent model selection
# as other commands - automatically choosing based on:
# - Availability (respects rate limits)
# - Cost (prefers free models)
# - Context window (needs space for diffs)
# - Speed (fast responses for better UX)
```

## Real-World Impact: 100% Autonomy

With these Git operations, gptcode now handles **all 64 autonomy scenarios**:

| Capability | Scenarios | Status |
|-----------|-----------|--------|
| Basic editing | 15 | ✅ 100% |
| Testing & coverage | 8 | ✅ 100% |
| Refactoring | 12 | ✅ 100% |
| Documentation | 6 | ✅ 100% |
| **Git operations** | **8** | ✅ **100%** |
| **Conflict resolution** | **5** | ✅ **100%** |
| Database migrations | 6 | ✅ 100% |
| Security scanning | 4 | ✅ 100% |

**Total: 64/64 (100%)**

## Under the Hood: How It Works

### Conflict Resolution Algorithm

```go
func (r *Resolver) ResolveFile(ctx context.Context, path string) error {
    // 1. Parse conflict markers
    conflicts := r.parseConflicts(path)
    
    // 2. Get context for each conflict
    for _, conflict := range conflicts {
        ours := conflict.OurVersion
        theirs := conflict.TheirVersion
        base := r.getCommonAncestor(path, conflict.LineRange)
        
        // 3. Build prompt with full context
        prompt := fmt.Sprintf(`
Conflict in %s at lines %d-%d:

Base version (common ancestor):
%s

Our changes:
%s

Their changes:
%s

Generate resolved code that preserves intent of both changes.
`, path, conflict.Start, conflict.End, base, ours, theirs)
        
        // 4. LLM generates resolution
        resp, err := r.llm.Chat(ctx, llm.ChatRequest{
            SystemPrompt: "You are a Git expert that resolves merge conflicts...",
            UserPrompt:   prompt,
            Model:        r.model,
        })
        
        // 5. Apply resolution
        resolved := resp.Content
        conflict.Resolution = resolved
    }
    
    // 6. Write resolved file
    return r.writeResolvedFile(path, conflicts)
}
```

### Git Bisect Implementation

```go
func (g *GitBisect) Run(ctx context.Context, good, bad, testCmd string) (string, error) {
    // 1. Start Git bisect
    exec.Command("git", "bisect", "start").Run()
    exec.Command("git", "bisect", "good", good).Run()
    exec.Command("git", "bisect", "bad", bad).Run()
    
    // 2. Binary search
    for {
        // Run test at current commit
        err := exec.Command("sh", "-c", testCmd).Run()
        
        if err != nil {
            exec.Command("git", "bisect", "bad").Run()
        } else {
            exec.Command("git", "bisect", "good").Run()
        }
        
        // Check if done
        output, _ := exec.Command("git", "bisect", "log").Output()
        if strings.Contains(string(output), "is the first bad commit") {
            break
        }
    }
    
    // 3. Get breaking commit
    result, _ := exec.Command("git", "bisect", "view", "--no-pager").Output()
    commit := parseCommitHash(result)
    
    // 4. LLM analyzes the breaking commit
    diff, _ := exec.Command("git", "show", commit).Output()
    
    resp, err := g.llm.Chat(ctx, llm.ChatRequest{
        SystemPrompt: "You are a Git expert analyzing a bug-introducing commit...",
        UserPrompt: fmt.Sprintf("Analyze this breaking commit:\n\n%s", diff),
        Model: g.model,
    })
    
    // 5. Clean up and return
    exec.Command("git", "bisect", "reset").Run()
    
    return fmt.Sprintf("Breaking commit: %s\n\nAnalysis:\n%s", 
        commit, resp.Content), nil
}
```

## Best Practices

### When to Use Each Command

**Git Bisect:** When you know a test is failing but don't know which commit broke it
```bash
gptcode git bisect --bad HEAD --good v1.0.0 --test "go test ./..."
```

**Cherry-Pick:** When you need a specific commit from another branch
```bash
gptcode git cherry-pick feature/abc123
```

**Rebase:** When syncing your feature branch with main
```bash
gptcode git rebase main
```

**Squash:** Before creating a PR to clean up commit history
```bash
gptcode git squash --count 8
```

**Reword:** When you have commits with poor messages
```bash
gptcode git reword HEAD~3  # Reword last 3 commits
```

**Resolve Conflicts:** Anytime you encounter merge conflicts
```bash
gptcode git resolve-conflicts
```

### Safety Features

All commands include safety checks:
- **Dirty working directory detection** - Won't run with uncommitted changes
- **Branch validation** - Confirms you're on the right branch
- **Backup creation** - Stores original state before operations
- **Rollback support** - Easy undo if something goes wrong
- **Dry-run mode** - Preview changes before applying

```bash
gptcode git squash --count 5 --dry-run
gptcode git rebase main --backup
```

## Future Enhancements

Coming soon:
- **Interactive rebase** - AI-suggested commit reorganization
- **Automatic PR creation** - From commit analysis
- **Conflict prediction** - Before merge/rebase
- **History analysis** - Find patterns in commit history
- **Smart branch naming** - Based on changes

## Try It Now

Update to the latest version:
```bash
gptcode upgrade
```

Or install for the first time:
```bash
go install github.com/jadercorrea/gptcode@latest
```

Full documentation: [Git Operations Guide](/docs/guides/git-operations.html)

## Summary

With advanced Git operations, gptcode achieves **100% autonomy across 64 real-world scenarios**:

✅ Automated bug hunting with `git bisect`  
✅ Intelligent conflict resolution for merge/rebase/cherry-pick  
✅ AI-generated commit messages for squash  
✅ Professional commit rewording  
✅ Zero-downtime database migrations  
✅ Complete Git workflow automation  

The AI coding assistant that was already helping you write code can now handle the full development workflow autonomously—from bug hunting to conflict resolution to commit history management.

**The future of Git is autonomous. And it's here today.**

---

*Questions or feedback? Open an issue on [GitHub](https://github.com/jadercorrea/gptcode) or join our [Discord](https://discord.gg/gptcode).*
