# Contributing to GPTCode

Thank you for considering contributing to GPTCode. The project explores
repository-centered, verifiable AI coding workflows.

## Code of Conduct

Keep technical discussion respectful, specific, inclusive, and evidence-based.

## How Can I Contribute?

### Reporting Bugs

Before creating a bug report:
1. Check [existing issues](https://github.com/jadercorrea/gptcode/issues)
2. Use the latest version of GPTCode

When reporting:
- Use a clear, descriptive title
- Describe exact steps to reproduce
- Include your OS and Go version
- Include relevant config files (`~/.gptcode/setup.yaml`)
- Paste error messages and logs

### Suggesting Features

We love feature ideas! Before suggesting:
1. Check [Discussions](https://github.com/jadercorrea/gptcode/discussions) for similar ideas
2. Explain how it strengthens explicit workflows, repository context, or executable verification

When suggesting:
- Use a clear, descriptive title
- Explain the problem you're solving
- Describe the solution you envision
- Describe how the behavior can be verified

### Pull Requests

1. **Fork and clone**
   ```bash
   git clone https://github.com/YOUR_USERNAME/gptcode
   cd gptcode
   ```

2. **Create a branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**
   - Follow existing code style
   - Add tests for new functionality
   - Update documentation if needed

4. **Test your changes**
   ```bash
   make verify
   ```

5. **Commit with clear messages**
   ```bash
   git commit -m "Add feature: brief description
   
   - Detail 1
   - Detail 2"
   ```

6. **Push and create PR**
   ```bash
   git push origin feature/your-feature-name
   ```

## Development Setup

### Prerequisites

- Go 1.24
- A provider key or Ollama only when exercising a real model integration

### Project Structure

```
gptcode/
├── cmd/gptcode/          # CLI entry point
├── internal/
│   ├── config/           # Configuration loading
│   ├── llm/              # LLM provider implementations
│   ├── catalog/          # Model discovery and management
│   ├── modes/            # Chat, Research, Plan, Implement
│   ├── agents/           # Router, Query, Editor, Research agents
│   └── tools/            # Tool implementations (read_file, etc)
└── docs/                 # Documentation and blog
```

### Building

**IMPORTANT:** The main entry point is `cmd/gptcode/main.go`, NOT `main.go` in the root.
The root `main.go` is in `.gitignore` and should never exist.

```bash
# Recommended: Use Makefile
make build          # Builds to bin/gptcode
make install        # Builds and installs to $GOPATH/bin

# Alternative: Direct Go commands
go build -o bin/gptcode ./cmd/gptcode
go install ./cmd/gptcode

# Run the same quality gate used by CI
make verify
```

To run that gate automatically before each commit:

```bash
git config core.hooksPath .githooks
```

### Testing LLM Providers

To test without spending money:
- Use Ollama (free local models)
- Use small Groq models (llama-3.1-8b-instant is $0.05/1M)
- Mock responses in tests

### Adding a New LLM Provider

1. Add provider constants to `internal/llm/llm.go`
2. Implement `Provider` interface in new file
3. Add to provider factory in `llm.go`
4. Update setup wizard in `internal/config/setup.go`
5. Add configuration docs
6. Test with real API

Example PR: See how Groq provider was added

### Adding a New Agent

1. Create agent struct in `internal/agents/`
2. Implement agent-specific prompt building
3. Wire into appropriate mode (`internal/modes/`)
4. Add configuration support
5. Add tests
6. Document usage

## Code Style

- Follow standard Go conventions (`gofmt`, `golint`)
- Use meaningful variable names
- Comment public functions and complex logic
- Keep functions focused and small
- Prefer explicit over clever

## Documentation

- Update README.md for user-facing changes
- Add/update blog posts for new features
- Include code examples
- Keep language clear and accessible

## Commit Messages

Good:
```
Add Ollama model auto-installation

- Scrape ollama.com for available models
- Add installed flag to track local models
- Prompt user to install when selecting unavailable model
```

Bad:
```
fix stuff
```

## Testing

- Add unit tests for new functions
- Test with multiple LLM providers when relevant
- Test both CLI and Neovim integration
- Verify cost implications of changes

## Questions?

- Ask in [Discussions](https://github.com/gptcode-cloud/cli/issues)
- Tag issues with `question` label
- Reach out to maintainers

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for helping make AI coding assistance affordable for everyone! 🐺
