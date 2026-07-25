# Advanced Git Operations with GPTCode

GPTCode provides AI-powered Git operations that automate complex workflows and resolve conflicts intelligently.

## Table of Contents

- [Git Bisect](#git-bisect)
- [Cherry-Pick](#cherry-pick)
- [Rebase](#rebase)
- [Squash Commits](#squash-commits)
- [Reword Commits](#reword-commits)
- [Merge Conflict Resolution](#merge-conflict-resolution)

## Git Bisect

Automatically find which commit introduced a bug using binary search.

### Basic Usage

```bash
gptcode git bisect <good-commit> <bad-commit>
```

### Example

```bash
gptcode git bisect v1.0.0 HEAD
```

**What it does:**
1. Starts Git bisect between the good and bad commits
2. Automatically runs `go test ./...` on each commit
3. Marks commits as good/bad based on test results
4. Uses LLM to analyze the breaking commit when found
5. Provides detailed analysis of what changed and why it broke

### Output

```
🔍 Starting bisect: v1.0.0 (good) ... HEAD (bad)

📍 Iteration 1: Testing commit...
✅ Tests passed - marking as good

📍 Iteration 2: Testing commit...
❌ Tests failed - marking as bad

🎯 Found the culprit!

📊 Analysis:
The commit changed the Multiply function to add 1 to the result,
which breaks the mathematical correctness...
```

### Notes

- Currently runs `go test ./...` by default (Go projects only)
- Automatically resets bisect when done
- Provides LLM-powered analysis of the breaking commit

---

## Cherry-Pick

Apply commits from one branch to another with intelligent conflict resolution.

### Basic Usage

```bash
gptcode git cherry-pick <commit> [<commit>...]
```

### Example

```bash
# Cherry-pick a single commit
gptcode git cherry-pick abc123

# Cherry-pick multiple commits
gptcode git cherry-pick abc123 def456 ghi789
```

**What it does:**
1. Attempts to cherry-pick each commit in order
2. If conflicts occur, uses LLM to resolve them automatically
3. Stages resolved files and continues cherry-pick
4. Provides progress for each commit

### Output

```
🍒 Cherry-picking 2 commit(s)...

[1/2] Cherry-picking abc123
✅ Applied successfully

[2/2] Cherry-picking def456
⚠️  Conflicts detected - resolving...
  Resolving src/main.go...
✅ Conflicts resolved and applied

✅ All commits cherry-picked successfully
```

---

## Rebase

Rebase with AI-powered conflict resolution.

### Basic Usage

```bash
gptcode git rebase [branch]
```

### Example

```bash
# Rebase onto main
gptcode git rebase main

# Interactive rebase
gptcode git rebase --interactive HEAD~5
```

**What it does:**
1. Starts rebase onto target branch
2. Detects conflicts automatically
3. Uses LLM to resolve conflicts intelligently
4. Continues rebase after resolution

### Output

```
🔄 Rebasing onto main...
⚠️  Conflicts detected - resolving...
  Resolving src/auth.go...
✅ Conflicts resolved and applied

✅ Rebase completed with conflict resolution
```

---

## Squash Commits

Combine multiple commits into one with an AI-generated commit message.

### Basic Usage

```bash
gptcode git squash <base-commit>
```

### Example

```bash
# Squash last 3 commits
gptcode git squash HEAD~3

# Squash all commits since a specific commit
gptcode git squash abc123
```

**What it does:**
1. Shows commits to be squashed
2. Analyzes commit messages and diffs
3. Generates a professional commit message via LLM
4. Performs soft reset and creates squashed commit

### Output

```
📦 Squashing 3 commit(s)...

  • 5f786e3 Add User model
  • cf1fb90 Fix multiply bug
  • 35b4aca Add proper tests

📝 Generated commit message:
Fix calculator bugs and add tests

* Corrected Multiply function to return accurate results
* Added unit tests for Add, Subtract, Multiply, and Divide functions to ensure calculator accuracy

✅ Commits squashed successfully
```

### Generated Message Format

The LLM generates commit messages following best practices:
- **Subject line**: Concise summary (50 chars or less)
- **Blank line**: Separator
- **Body**: Detailed explanation with bullet points
- **Conventional commits**: feat:, fix:, etc. when appropriate

---

## Reword Commits

Get AI-powered suggestions to improve commit messages.

### Basic Usage

```bash
gptcode git reword <commit>
```

### Example

```bash
# Improve the last commit message
gptcode git reword HEAD

# Improve a specific commit
gptcode git reword abc123
```

**What it does:**
1. Reads the current commit message
2. Analyzes the commit diff
3. Generates an improved message following best practices
4. Suggests how to apply it

### Output

```
📝 Suggested commit message:
Fix calculator multiplication and add unit tests

* Correct Multiply function to return accurate results
* Add unit tests for basic arithmetic operations to ensure calculator accuracy

💡 To apply: git commit --amend -m "<message>"
```

### Notes

- Does not automatically apply the change
- You can review the suggestion first
- Use `git commit --amend` to apply manually

---

## Merge Conflict Resolution

Automatically resolve all merge conflicts using AI.

### Basic Usage

```bash
gptcode merge resolve
```

### Example

```bash
# After a failed merge
git merge feature-branch
# CONFLICT (content): Merge conflict in src/main.go

gptcode merge resolve
```

**What it does:**
1. Detects all files with merge conflicts
2. Analyzes conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`)
3. Uses LLM to intelligently merge both versions
4. Validates resolution (ensures no conflict markers remain)
5. Stages resolved files

### Output

```
🔍 Found 2 file(s) with conflicts

📝 Resolving src/main.go...
   ✅ Resolved and staged

📝 Resolving src/auth.go...
   ✅ Resolved and staged

✅ All conflicts resolved
💡 Review changes with: git diff --cached
💡 Commit with: git commit
```

### Conflict Resolution Strategy

The LLM analyzes both versions (HEAD and incoming) and:
1. Keeps non-conflicting changes from both sides
2. Merges semantic changes when possible
3. Prioritizes code correctness and functionality
4. Removes all conflict markers

### Important Notes

⚠️ **Always review resolved conflicts!**

- The LLM makes intelligent decisions but may not always be correct
- Use `git diff --cached` to review changes before committing
- Test your code after resolution

---

## Configuration

All Git operations use your default LLM configuration from `~/.gptcode/setup.yaml`.

### Custom Model

You can specify a different model for Git operations:

```bash
gptcode git squash HEAD~3 --model claude-3-5-sonnet-20241022
```

### Supported Models

- **Groq**: llama-3.3-70b-versatile, llama-3.1-8b-instant
- **Ollama**: qwen3-coder:latest, llama3.1:8b
- **Anthropic**: claude-3-5-sonnet-20241022
- **OpenAI**: gpt-4o, gpt-4o-mini

---

## Best Practices

### Git Bisect

1. Make sure your test suite is reliable
2. Use specific commits or tags for boundaries
3. Review the LLM analysis but verify yourself

### Cherry-Pick

1. Cherry-pick in chronological order
2. Review resolved conflicts before continuing
3. Run tests after cherry-picking multiple commits

### Squash

1. Squash related commits together
2. Review the generated message
3. Edit if needed before accepting

### Conflict Resolution

1. **Always review** resolved conflicts
2. Run tests after resolution
3. Use `git diff --cached` to see what changed
4. Don't blindly trust AI - understand the resolution

---

## Troubleshooting

### Bisect doesn't work for my project

**Issue**: Default test command (`go test ./...`) doesn't work

**Solution**: Bisect currently supports Go projects only. For other languages, use manual bisect.

### Conflict resolution failed

**Issue**: LLM couldn't resolve the conflict

**Solution**:
1. Check the error message
2. Resolve manually: `git mergetool` or edit files
3. Continue: `git add <file> && git commit`

### Squash generated bad commit message

**Issue**: The message doesn't accurately describe changes

**Solution**:
1. The commit was already created
2. Amend it: `git commit --amend`
3. Edit the message to your liking

---

## Examples

### Complete Workflow

```bash
# Feature development with squashing
git checkout -b feature-auth
# ... make multiple commits ...

# Squash into one clean commit
gptcode git squash main

# Rebase onto main with conflict resolution
gptcode git rebase main

# Create PR
git push origin feature-auth
```

### Bug Hunting

```bash
# Find the breaking commit
gptcode git bisect v1.0.0 HEAD

# Cherry-pick the fix to release branch
git checkout release-1.0
gptcode git cherry-pick <fix-commit>
```

### Cleaning Up History

```bash
# Improve recent commit messages
gptcode git reword HEAD~2
gptcode git reword HEAD~1
gptcode git reword HEAD

# Squash related changes
gptcode git squash HEAD~5
```

---

## FAQ

**Q: Is conflict resolution safe?**

A: The LLM makes intelligent decisions, but you should **always review** the resolution before committing. Use `git diff --cached` to see changes.

**Q: Can I undo a squash?**

A: Yes! The original commits still exist. Use `git reflog` to find them and `git reset --hard <commit>` to restore.

**Q: Does bisect work with other test commands?**

A: Currently bisect runs `go test ./...` by default. Support for custom test commands is coming soon.

**Q: What models work best for Git operations?**

A: We recommend:
- **Best**: claude-3-5-sonnet-20241022 (most accurate)
- **Fast**: llama-3.3-70b-versatile (Groq, very cheap)
- **Free**: qwen3-coder:latest (Ollama, local)

---

## Related Commands

- `gptcode coverage` - Analyze test coverage
- `gt review` - Code review with AI
- `gptcode gen changelog` - Generate CHANGELOG from commits
- `gt docs update` - Update README based on changes

---

## Feedback

Found a bug or have a suggestion? [Open an issue](https://github.com/jadercorrea/gptcode/issues/new)!
