# Universal Context Management

**The AI-agnostic way to manage project context across any AI assistant.**

## The Problem

You're working on a large project with 20+ microservices. Every time you start a new AI assistant session (Warp, Cursor, ChatGPT, Claude), you face the same questions:

- "What's the architecture?"
- "What tech stack do we use?"
- "What should I work on next?"
- "How do services communicate?"

You copy-paste the same context repeatedly. When architecture changes, you update it in 5 different places. Each team member has their own version. New developers spend days understanding the system.

**Sound familiar?**

## The Solution

GPTCode's Universal Context Layer provides a **single source of truth** for project context that:

- ✅ Works with **ANY AI assistant** (Warp, Cursor, Claude, Gemini, Cline, Aider, etc)
- ✅ **Version-controlled** alongside your code
- ✅ **Team-shared** via git
- ✅ **Auto-syncs** to integration formats
- ✅ **Separates concerns**: "what to do" vs "how things work"

## Quick Start

### 1. Initialize Context Layer

```bash
cd ~/your-project
gt context init
```

This creates:
```
.gptcode/
  context/
    shared.md   → Architecture, stack, patterns
    next.md     → Immediate next tasks
    roadmap.md  → Long-term roadmap
  config.yml    → Integration configuration
  .gitignore
```

### 2. Edit Your Context

```bash
vi .gptcode/context/shared.md
```

Example `shared.md`:
```markdown
# Project Context

## Architecture
Microservices architecture with 20+ services:
- API Gateway (Kong) → routes to services
- BFF (Backend For Frontend) → aggregates data
- Services communicate via Redis pub/sub
- PostgreSQL for persistent data

## Stack
- **Backend**: Elixir/Phoenix, Node.js
- **Frontend**: React, TypeScript
- **Infrastructure**: AWS (ECS, RDS), Terraform
- **Monitoring**: Datadog, Sentry

## Patterns
- REST APIs follow JSON:API spec
- All services use structured logging (JSON)
- Feature flags via LaunchDarkly
- Database migrations with Ecto/Prisma

## Development
```bash
# Start all services
docker-compose up

# Run tests
mix test          # Elixir
npm test          # Node.js

# Deploy
terraform apply
```
```

### 3. Use Your Context

**Option A: Auto-sync to integrations**
```bash
gt context sync
# ✅ Synced to WARP.md (Warp uses it automatically)
# ✅ Synced to .cursor/docs/ (Cursor reads it)
```

**Option B: Copy to clipboard**
```bash
gt context export clipboard
# ✅ Context copied - paste into any AI chat
```

**Option C: Show in terminal**
```bash
gt context show          # Show all
gt context show shared   # Show specific
```

## Commands Reference

### `gt context init`
Initialize `.gptcode/` directory structure.

**Example:**
```bash
cd ~/project
gt context init
```

**Output:**
```
🚀 Initializing GPTCode context layer...
✅ Context layer initialized!

📁 Structure created:
  .gptcode/
    context/
      shared.md   - Technical context
      next.md     - Next tasks
      roadmap.md  - Roadmap
    config.yml    - Configuration

📝 Next steps:
  1. Edit context files: vi .gptcode/context/shared.md
  2. Show context: gt context show
  3. Export for use: gt context export clipboard
```

### `gt context show [type]`
Display context content.

**Examples:**
```bash
gt context show          # Show all contexts
gt context show shared   # Show only shared.md
gt context show next     # Show only next.md
gt context show roadmap  # Show only roadmap.md
```

### `gt context add <type> <content>`
Append content to a context file.

**Examples:**
```bash
gt context add shared "## New Section\nContent here"
gt context add next "- [ ] Implement feature X"
gt context add roadmap "### Q2 2025\n- Launch mobile app"
```

### `gt context sync`
Sync context to integration formats.

**What it does:**
1. Reads `.gptcode/config.yml`
2. For each enabled integration:
   - **Warp**: Writes `WARP.md` (shared + next)
   - **Cursor**: Copies all files to `.cursor/docs/`
3. Reports status

**Example:**
```bash
gt context sync

# Output:
🔄 Syncing context to integrations...
✅ Synced to WARP.md
✅ Synced to 1 integration(s)
```

**Configuration** (`.gptcode/config.yml`):
```yaml
integrations:
  warp:
    enabled: true
    rule_path: WARP.md
  cursor:
    enabled: false    # Enable if using Cursor
    doc_path: .cursor/docs
```

### `gt context export <format>`
Export context to specific format or clipboard.

**Examples:**
```bash
# Copy to clipboard (macOS)
gt context export clipboard
# ✅ Context copied to clipboard
# 📋 1,234 characters ready to paste

# Print in Warp format
gt context export warp

# Print in Cursor format
gt context export cursor
```

## Use Cases

### 1. Large Monorepos

**Problem:** 20+ services, each with different patterns. New AI session = 10 minutes explaining everything.

**Solution:**
```bash
.gptcode/context/shared.md  # Documents all 20 services
gt context sync         # WARP.md always up-to-date
```

**Result:** Every Warp session instantly knows your entire architecture.

### 2. Team Collaboration

**Problem:** Each developer maintains their own AI context. Inconsistent answers, outdated information.

**Solution:**
```bash
# Developer A updates context
echo "## New: Redis Caching\nAdded Redis..." >> .gptcode/context/shared.md
git commit -am "Add Redis context"
git push

# Developer B receives update
git pull
gt context sync  # Local tools updated automatically
```

**Result:** Team shares single source of truth, always in sync.

### 3. Multi-Tool Workflows

**Problem:** Use Warp for terminal, Cursor for IDE, Claude web for design. Different context in each.

**Solution:**
```yaml
# .gptcode/config.yml
integrations:
  warp:
    enabled: true
  cursor:
    enabled: true
```

```bash
gt context sync              # Updates Warp & Cursor
gt context export clipboard  # For Claude web chat
```

**Result:** Same context everywhere, zero copy-paste.

### 4. Onboarding

**Problem:** New developer takes 2 weeks to understand codebase. Asks same questions repeatedly.

**Solution:**
```bash
# New dev joins
git clone repo
cd repo
gt context show  # ← Instant architecture overview

# Start using AI assistants
gt context sync  # ← Tools have full context immediately
```

**Result:** Productive from day 1.

### 5. Context Evolution

**Problem:** Architecture changes. Context in Slack, Notion, README all outdated.

**Solution:**
```bash
# Architecture changes
vi .gptcode/context/shared.md  # Update in ONE place
gt context sync           # Propagate everywhere
git commit -m "Update: migrated to microservices"
```

**Result:** Context evolves with code. Git history shows evolution.

## Best Practices

### 1. Keep Contexts Focused

**Good:**
```markdown
## Stack
- Backend: Elixir/Phoenix
- Frontend: React + TypeScript
- DB: PostgreSQL
```

**Bad:**
```markdown
## Stack
We use Elixir because in 2019 we evaluated multiple options including
Ruby on Rails, Node.js, Python Django, and after considering factors
like performance, concurrency, fault-tolerance...
[5 paragraphs later]
```

**Rule:** Context should be **quick reference**, not a novel.

### 2. Separate Concerns

**shared.md** → Stable technical context
- Architecture
- Stack
- Patterns
- Development setup

**next.md** → Current priorities (changes weekly)
- Tasks in progress
- This week's focus
- Immediate backlog

**roadmap.md** → Long-term vision (changes quarterly)
- Q1/Q2/Q3 goals
- Future plans
- Ideas

### 3. Update Context with Code

```bash
# Feature PR that adds Redis
git commit -m "feat: Add Redis caching

Implementation:
- Added Redis client in lib/redis.ex
- Updated caching strategy

Context:
- Updated .gptcode/context/shared.md with Redis layer
- Removed 'add caching' from .gptcode/context/next.md
"
```

### 4. Review Context in PRs

Just like code review:
```markdown
# PR Description
## Code Changes
- Added Redis caching layer

## Context Changes
- Updated shared.md: Added Redis section
- Updated next.md: Removed caching task

## Reviewers: Please verify context is accurate
```

### 5. Automate Sync

**Git Hook** (`.git/hooks/post-merge`):
```bash
#!/bin/bash
# Auto-sync context after git pull
if command -v gptcode &> /dev/null; then
    gt context sync --quiet
fi
```

## Configuration

### Integration: Warp

**Default config:**
```yaml
integrations:
  warp:
    enabled: true
    rule_path: WARP.md
```

**How it works:**
1. `gt context sync` combines `shared.md` + `next.md`
2. Writes to `WARP.md` in project root
3. Warp Agent automatically loads `WARP.md` as project rules

**Custom path:**
```yaml
warp:
  enabled: true
  rule_path: docs/WARP_RULES.md  # Custom location
```

### Integration: Cursor

**Default config:**
```yaml
integrations:
  cursor:
    enabled: false  # Disabled by default
    doc_path: .cursor/docs
```

**Enable:**
```yaml
cursor:
  enabled: true
  doc_path: .cursor/docs
```

**How it works:**
1. `gt context sync` copies each file individually:
   - `.gptcode/context/shared.md` → `.cursor/docs/shared.md`
   - `.gptcode/context/next.md` → `.cursor/docs/next.md`
   - `.gptcode/context/roadmap.md` → `.cursor/docs/roadmap.md`
2. Cursor reads all `.md` files in `.cursor/docs/`

### Auto-load Configuration

Control which contexts are included in sync/export:

```yaml
auto_load:
  - shared   # Always include
  - next     # Always include
  # roadmap is excluded from Warp sync (too long)
```

**Effect:**
- `gt context sync` → Uses `auto_load` list
- `gt context export warp` → Uses `shared` + `next`
- `gt context export cursor` → Uses all three
- `gt context show` → Shows requested types

## Version Control

### What to Commit

**✅ Commit:**
```
.gptcode/
  context/         # Source of truth
  config.yml       # Integration config
  .gitignore       # Ignore rules

WARP.md            # Generated, but commit it
.cursor/docs/      # Generated, but commit it
```

**Why commit generated files?**
- Team members without GPTCode CLI still get context
- Integrations work immediately after `git clone`
- CI/CD can validate context (future feature)

### What NOT to Commit

Add to `.gptcode/.gitignore`:
```gitignore
# Temporary files
sessions/
diffs/
*.tmp
```

### Git Workflow

**Adding context:**
```bash
vi .gptcode/context/shared.md
gt context sync
git add .gptcode/ WARP.md
git commit -m "docs: Update architecture context"
git push
```

**Receiving updates:**
```bash
git pull
gt context sync  # Update local integrations
```

**Merge conflicts:**
```bash
# If .gptcode/context/shared.md conflicts
vi .gptcode/context/shared.md  # Resolve conflict
git add .gptcode/context/shared.md
git commit
gt context sync           # Regenerate WARP.md
git add WARP.md
git commit --amend
```

## Advanced Usage

### Multiple Projects

Use different contexts per project:

```bash
cd ~/project-a
gt context init
gt context sync  # → project-a/WARP.md

cd ~/project-b
gt context init
gt context sync  # → project-b/WARP.md
```

Warp Agent loads correct `WARP.md` based on current directory.

### Nested Projects

Context search walks up directory tree:

```bash
/monorepo/
  .gptcode/              # Root context
  service-a/
    # Inherits root context
  service-b/
    .gptcode/            # Service-specific context (overrides root)
```

### CI/CD Integration

**Validate context in CI:**
```yaml
# .github/workflows/validate.yml
- name: Validate context
  run: |
    gt context show
    gt context sync --dry-run  # Future: validate without writing
```

**Auto-sync on deploy:**
```bash
# deploy.sh
gt context add next "- [x] Deployed v1.2.3 to production"
git commit -am "Update deployment status"
```

## Troubleshooting

### "`.gptcode` directory not found"

**Error:**
```bash
$ gt context show
Error: .gptcode directory not found (run 'gt context init')
```

**Solution:**
```bash
cd /path/to/project/root
gt context init
```

### "Clipboard export failed"

**Error:**
```bash
$ gt context export clipboard
Error: failed to copy to clipboard (pbcopy not available)
```

**Solution:**
- **macOS**: `pbcopy` should be available by default
- **Linux**: Install `xclip` or `xsel`
- **Windows**: Use `gt context export warp` and copy output

### Sync Not Working

**Check config:**
```bash
cat .gptcode/config.yml
```

**Verify enabled:**
```yaml
integrations:
  warp:
    enabled: true  # ← Must be true
```

**Run sync with verbose output:**
```bash
gt context sync
# Should show:
# ✅ Synced to WARP.md
```

## FAQ

### Do I need GPTCode CLI to use the context?

**No.** The context files are plain Markdown. Anyone can:
- Read `.gptcode/context/*.md` directly
- Use generated `WARP.md` in Warp
- Use `.cursor/docs/` in Cursor

GPTCode CLI just makes it easier to manage.

### What if I don't use Warp or Cursor?

Use `gt context export clipboard` to copy context for:
- ChatGPT web
- Claude web
- Gemini web
- Any AI chat interface

### Can I have multiple context files?

Currently: `shared.md`, `next.md`, `roadmap.md`.

**Future versions** may support custom files. File an issue if you need this.

### Does this work with private repos?

**Yes.** Everything is local files. No data sent to GPTCode servers.

### How is this different from `.cursorrules` or `WARP.md`?

**Those are tool-specific.** GPTCode Context Layer is **tool-agnostic**:

- ✅ Edit in ONE place (`.gptcode/context/`)
- ✅ Sync to ALL tools (`gt context sync`)
- ✅ Version-controlled evolution
- ✅ Team-shared via git

## Next Steps

- **Try it:** `gt context init`
- **Blog post:** [Why Your AI Needs Context Management](link)
- **Examples:** [Real-world context examples](link)
- **Community:** Share your context setup in Discussions

---

**Questions? Issues?** Open an issue on [GitHub](https://github.com/jadercorrea/gptcode/issues)
