---
layout: post
title: "Chat REPL: Conversational Coding with Context Memory"
date: 2025-12-04
author: Jader Correa
description: "A persistent REPL with conversation history, file context awareness, and token management. Natural follow-up questions that build on previous context."
tags: [features, ux, chat, repl]
---

# Chat REPL: Conversational Coding with Context Memory

We've covered [context optimization]({% post_url 2025-11-14-context-engineering-for-real-codebases %}) and [smart file selection]({% post_url 2025-12-03-dependency-graph-context-optimization %}). But what about **maintaining context across multiple questions**?

## The Problem: One-Shot Interactions

Traditional AI coding assistants are conversational... sort of.

**The issue**:
```bash
$ tool "explain this function"
[explanation]

$ tool "now refactor it"
[error: what function?]
```

Each invocation starts fresh. No memory, no context, no follow-up.

**You end up**:
- Repeating context in every query
- Copy-pasting code between prompts
- Losing thread of the conversation
- Forgetting what you asked 3 questions ago

## The Solution: Chat REPL

GPTCode's `gt chat` is a **persistent REPL** (Read-Eval-Print Loop) with:
- Conversation history
- File context awareness
- Token management
- Save/load conversations

### Basic Usage

```bash
$ gt chat

> explain the authentication flow
[GPTCode reads auth.go, middleware.go, session.go]
The authentication flow works as follows:
1. User submits credentials to /login
2. Middleware validates JWT token
3. Session stored in Redis...

> how is the JWT token generated?
[GPTCode remembers context, focuses on token logic]
The JWT token is generated in auth/token.go:
- Uses HMAC-SHA256 signing
- Includes user_id, email, exp claims
- Token expires in 24 hours...

> add refresh token support
[GPTCode knows you're extending the JWT system]
I'll add refresh token functionality:
1. Create refresh_token table
2. Generate long-lived refresh token (30 days)
3. Add /refresh endpoint
4. Update login to return both tokens...
```

**Key difference**: GPTCode remembers the conversation. Question 2 builds on Question 1. Question 3 knows we're working on auth + JWT.

## Features

### 1. Conversation History

Tracks all messages with metadata:

```json
{
  "role": "user",
  "content": "explain the authentication flow",
  "timestamp": "2025-12-04T10:30:00Z",
  "token_count": 8
}
```

**Benefits**:
- Context preserved across questions
- No need to repeat yourself
- Natural follow-up questions

### 2. File Context Awareness

Automatically includes relevant files based on:
- Current directory
- Recent git changes
- Mentioned file names
- Dependency graph

**Example**:
```bash
> fix bug in auth/handler.go
```

GPTCode sees:
- `auth/handler.go` (explicitly mentioned)
- `auth/middleware.go` (dependency)
- `models/user.go` (used by handler)
- Recent changes in git diff

### 3. Token Management

Tracks context window usage:

```bash
> /context
Context Manager Status:
  Total Messages: 8
  Total Tokens: 2,450 / 8,000 (30.6%)
  Oldest Message: 2025-12-04 10:25:03
  Recent File Updates: 3 minutes ago
```

**Automatic management**:
- Keeps last 50 messages (configurable)
- Drops oldest when exceeding limit
- Summarizes long messages
- Preserves important context

### 4. REPL Commands

| Command | Description |
|---------|-------------|
| `/exit`, `/quit` | Exit chat |
| `/clear` | Clear conversation history |
| `/save <file>` | Save conversation to file |
| `/load <file>` | Load previous conversation |
| `/context` | Show context statistics |
| `/files` | List files in context |
| `/history` | Show conversation history |
| `/help` | Show all commands |

### 5. Save/Load Conversations

Continue later where you left off:

```bash
> /save auth-discussion.json
Conversation saved to auth-discussion.json

# Later, different terminal session
$ gt chat
> /load auth-discussion.json
Conversation loaded from auth-discussion.json

> continue with the refresh token implementation
[GPTCode has full context from previous session]
```

## Real-World Workflow

### Scenario: Adding Feature to Existing Code

```bash
$ gt chat

> show me the current user registration flow
[GPTCode reads user.go, handler.go, validates.go]
Current registration:
1. POST /register with email, password
2. Validate email format
3. Hash password with bcrypt
4. Save to database
5. Send welcome email

> what validation rules exist?
[Remembers we're discussing registration]
Current validation:
- Email: Must be valid format
- Password: Min 8 chars
- No duplicate email check yet ⚠️

> add duplicate email validation
[Knows exact context: registration + validation]
I'll add duplicate check:
1. Before insert, query users by email
2. Return 409 Conflict if exists
3. Update validation error messages

[code changes here]

> also add password strength requirements
[Still in context of registration + validation]
Adding password strength:
- Min 8 chars (existing)
- Must include: uppercase, lowercase, number, special char
- Updated error messages

[code changes here]

> write tests for these validations
[Full context: registration + validation + recent changes]
I'll create tests:
- TestDuplicateEmailRejected
- TestWeakPasswordRejected
- TestValidPasswordAccepted

[test code here]

> /save user-registration-improvements.json
Conversation saved.
```

**Result**: 5 related changes, all with proper context. No repetition, natural flow.

## Initial Message Support

Start with a question immediately:

```bash
$ gt chat "explain the database schema"
[Processes question and stays open]
The database has 5 main tables:
...

> how is user data encrypted?
[Follow-up in same session]
```

**Non-interactive mode** (for CI/scripts):
```bash
$ gt chat "generate migration for new column" | tee migration.sql
[Processes and exits]
```

REPL detects TTY:
- TTY present → stays open for follow-up
- Piped/redirected → processes and exits

## Advanced: Context Control

### View Current Context

```bash
> /context
Context Manager Status:
  Total Messages: 12
  Total Tokens: 4,823 / 8,000 (60.3%)
  Cache Hit Rate: 71%
  Oldest Message: 2025-12-04 10:15:22
  Recent File Updates: 2 minutes ago
```

### Check Files in Context

```bash
> /files
Files in context:
  internal/auth/handler.go
  internal/auth/middleware.go
  internal/models/user.go
  config/security.go
```

### Clear and Start Fresh

```bash
> /clear
Conversation history cleared.

> [start new topic]
```

Useful when:
- Switching to unrelated task
- Context getting too large
- Want clean slate

## Technical Details

### Context Manager

Implements sliding window with smart truncation:

```go
type ContextManager struct {
    messages     []Message
    maxTokens    int  // 8000 default
    maxMessages  int  // 50 default
    fileContext  string
}

func (cm *ContextManager) AddMessage(role, content string, tokens int) {
    cm.messages = append(cm.messages, Message{
        Role: role,
        Content: content,
        Timestamp: time.Now(),
        TokenCount: tokens,
    })
    
    // Auto-truncate if needed
    if cm.getTotalTokens() > cm.maxTokens {
        cm.truncateOldest()
    }
}
```

### Token Estimation

Uses simple heuristic (will be improved):

```go
func estimateTokens(text string) int {
    // Rough estimate: 1 token ≈ 4 characters
    return len(text) / 4
}
```

**Future**: Use tiktoken for accurate counts.

### File Context Updates

Refreshes every 5 minutes or on demand:

```go
func (cm *ContextManager) UpdateFileContext() error {
    // Get git status
    changed, _ := getGitChangedFiles()
    
    // Get cwd files
    local, _ := listLocalFiles(".")
    
    // Build context string
    cm.fileContext = buildFileContext(changed, local)
    return nil
}
```

## Integration with Other Features

### Dependency Graph

Chat REPL uses graph to find relevant files:

```bash
> fix bug in authentication
[Graph selects: auth.go, middleware.go, session.go]
[Includes these in context automatically]
```

### Model Selection

Each message uses optimal model:

```bash
> explain this code
[Uses query agent: gemini-2.0-flash-exp:free]

> refactor it
[Uses editor agent: llama-3.3-70b-versatile]
```

### ML Intent Classification

Routes messages to right agent:

```bash
> how does caching work?
[ML: query intent → research agent]

> add Redis caching
[ML: editor intent → editor agent]
```

## Comparison: Chat REPL vs One-Shot

| Aspect | Chat REPL | One-Shot (traditional) |
|--------|-----------|------------------------|
| **Context** | Preserved | Lost |
| **Follow-up** | Natural | Requires full context |
| **Efficiency** | High (no repetition) | Low (repeat context) |
| **Token usage** | Optimized (shared context) | Wasteful (duplicate context) |
| **UX** | Conversational | Transactional |
| **Best for** | Exploration, debugging | Simple queries |

## Configuration

### Adjust Context Limits

```bash
# Max tokens (default: 8000)
gt config set defaults.chat_max_tokens 16000

# Max messages (default: 50)
gt config set defaults.chat_max_messages 100
```

**Higher limits** = more context, but slower and more expensive
**Lower limits** = less context, faster, cheaper

### Change Model

Uses default profile models, but can override:

```bash
gt chat --model gpt-4
```

## Best Practices

### 1. Use Descriptive First Message

```bash
# Good
gt chat "I'm working on the auth module, explain the JWT flow"

# Less good
gt chat "explain"
```

Better context → better responses.

### 2. Save Important Conversations

```bash
> /save feature-x-discussion-2025-12-04.json
```

Reference later or share with team.

### 3. Clear When Switching Topics

```bash
> /clear
```

Prevents context pollution.

### 4. Check Context Periodically

```bash
> /context
```

If at 80%+ tokens, consider `/clear` or `/save` and start fresh.

### 5. Use Initial Message for CI

```bash
gt chat "check if PR follows coding standards" < pr_diff.txt
```

## Keyboard Shortcuts

Based on readline library:

- `Ctrl+D` - Exit (EOF)
- `Ctrl+C` - Interrupt (shows hint)
- `Up/Down` - History navigation
- `Ctrl+A` - Start of line
- `Ctrl+E` - End of line
- `Ctrl+K` - Kill to end
- `Ctrl+U` - Kill to start

History saved to `~/.gptcode_history`.

## Error Handling

### Lost Connection

```bash
> explain database schema
Error: connection timeout

> [retry automatically]
```

Chat REPL stays open on errors.

### Invalid Command

```bash
> /invalid
Unknown command: /invalid (type /help for available commands)
```

### Context Overflow

```bash
[Warning: Context at 95% capacity (7600/8000 tokens)]
[Automatically dropping 3 oldest messages]
```

Transparent, automatic management.

## Future Enhancements

### 1. Voice Input

```bash
$ gt chat --voice
> [speak] "explain authentication"
```

### 2. Multi-File Editing

```bash
> refactor auth module
[GPTCode: I'll update 4 files, continue? y/n]
```

### 3. Interactive Diff Preview

```bash
> add logging
[Shows diff, ask for confirmation]
[Apply with /accept, reject with /reject]
```

### 4. Collaborative Sessions

```bash
> /share
Session URL: chu.dev/s/abc123
[Others can join and see conversation]
```

### 5. Semantic Search in History

```bash
> /search "password validation"
[Shows relevant messages from history]
```

## Getting Started

### 1. Start Chat

```bash
gt chat
```

### 2. Ask Questions

```bash
> explain how routing works
> where is the config loaded?
> fix the bug in handler.go
```

### 3. Use Commands

```bash
> /context
> /files
> /history
```

### 4. Save Work

```bash
> /save today-discussion.json
```

### 5. Exit

```bash
> /exit
```

## Community Examples

**Example 1**: Debugging Session
```
> why is the API returning 500?
> check the logs
> what's in error.log?
> the database connection is nil
> where is the database initialized?
> fix the initialization order
```
Result: 6-step debugging, all in context.

**Example 2**: Feature Development
```
> I want to add rate limiting
> show current middleware stack
> add rate limiter before auth
> configure it in security.go
> write tests
> /save rate-limiting.json
```
Result: Complete feature, documented conversation.

**Example 3**: Code Review
```
> /load pr-342-review.json
> continue reviewing the changes
> check error handling in handler.go
> suggest improvements
```
Result: Resumed previous review session.

## Summary

Chat REPL delivers:
- **Persistent context** (no repetition)
- **Natural follow-up** (conversational)
- **File awareness** (automatic relevance)
- **Token management** (optimized costs)
- **Save/load** (continuity)
- **REPL commands** (control)

Try it today:
```bash
gt chat
```

---

*Have questions about Chat REPL? Join our [GitHub Discussions](https://github.com/jadercorrea/gptcode/issues)*

## See Also

- [Dependency Graph](2025-12-03-dependency-graph-context-optimization) - Smart file selection
- [Context Engineering](2025-11-14-context-engineering-for-real-codebases) - Managing context windows
- [Complete Workflow Guide](2025-11-24-complete-workflow-guide) - End-to-end workflows
