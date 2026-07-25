package modes

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gptcode/internal/agents"
	"gptcode/internal/config"
	"gptcode/internal/llm"
	"gptcode/internal/output"

	"golang.org/x/term"
)

func RunResearch(args []string) error {
	question := ""
	if len(args) > 0 {
		question = strings.Join(args, " ")
	}

	if question == "" {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Fprintln(os.Stderr, "Research mode - Analyze codebase and external docs")
		fmt.Fprintln(os.Stderr, "\nWhat would you like to research?")
		fmt.Fprint(os.Stderr, "> ")
		if scanner.Scan() {
			question = scanner.Text()
		}
	}

	if question == "" {
		return fmt.Errorf("no research question provided")
	}

	setup, _ := config.LoadSetup()
	backendName := setup.Defaults.Backend
	backendCfg := setup.Backend[backendName]
	cwd, _ := os.Getwd()

	urls := extractURLs(question)
	var externalDocs string

	if len(urls) > 0 {
		fmt.Fprintf(os.Stderr, "⠋ Fetching external documentation...\n")

		var orchestrator *llm.OrchestratorProvider
		if backendCfg.Type == "ollama" {
			customExec := llm.NewOllama(backendCfg.BaseURL)
			orchestrator = llm.NewOrchestrator(backendCfg.BaseURL, backendName, customExec, backendCfg.DefaultModel)
		} else {
			customExec := llm.NewChatCompletion(backendCfg.BaseURL, backendName)
			orchestrator = llm.NewOrchestrator(backendCfg.BaseURL, backendName, customExec, backendCfg.DefaultModel)
		}

		researchAgent := agents.NewResearch(orchestrator)
		for _, url := range urls {
			docPrompt := fmt.Sprintf("Visit %s and summarize the key implementation details for: %s", url, question)
			docResult, err := researchAgent.Execute(context.Background(), []llm.ChatMessage{{Role: "user", Content: docPrompt}}, nil)
			if err == nil {
				externalDocs += fmt.Sprintf("\n\n## Documentation from %s\n\n%s", url, docResult)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "⠋ Analyzing codebase...\n")

	var customExec llm.Provider
	if backendCfg.Type == "ollama" {
		customExec = llm.NewOllama(backendCfg.BaseURL)
	} else {
		customExec = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	queryModel := backendCfg.GetModelForAgent("query")
	queryAgent := agents.NewQuery(customExec, cwd, queryModel)

	evidence, err := collectRepositoryEvidence(cwd)
	if err != nil {
		return fmt.Errorf("collect repository evidence: %w", err)
	}
	codebasePrompt := buildCodebaseResearchPrompt(question, evidence)
	researchCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	codebaseAnalysis, err := queryAgent.Execute(researchCtx, []llm.ChatMessage{{Role: "user", Content: codebasePrompt}}, nil)
	if err != nil {
		return fmt.Errorf("codebase analysis failed: %w", err)
	}
	if contradictsUnsafeConcurrencyEvidence(codebaseAnalysis, evidence) {
		correctionPrompt := codebasePrompt + `

Mandatory correction:
The inspected implementation contains a mutable map and no synchronization primitive.
The previous draft incorrectly called concurrent access safe. Rewrite the answer and
state explicitly that concurrent access is unsafe; tests and documentation do not
override the implementation evidence.`
		codebaseAnalysis, err = queryAgent.Execute(researchCtx, []llm.ChatMessage{{Role: "user", Content: correctionPrompt}}, nil)
		if err != nil {
			return fmt.Errorf("correct unsupported research conclusion: %w", err)
		}
		if contradictsUnsafeConcurrencyEvidence(codebaseAnalysis, evidence) {
			return fmt.Errorf("codebase analysis contradicted deterministic concurrency evidence")
		}
	}

	home, _ := os.UserHomeDir()
	researchDir := filepath.Join(home, ".gptcode", "research")
	_ = os.MkdirAll(researchDir, 0755)

	fullResearch := fmt.Sprintf(`# Research: %s

## Summary

%s

%s

## Generated
%s`, question, codebaseAnalysis, externalDocs, time.Now().Format("2006-01-02 15:04:05"))

	if term.IsTerminal(int(os.Stdout.Fd())) {
		rendered, err := output.RenderMarkdown(fullResearch)
		if err != nil {
			rendered = fullResearch
		}
		fmt.Println(output.Separator())
		fmt.Print(rendered)
		fmt.Println(output.Separator())
	} else {
		fmt.Println(fullResearch)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	sanitizedQuestion := sanitizeFilename(question)
	filename := fmt.Sprintf("%s_%s.md", timestamp, sanitizedQuestion)
	researchPath := filepath.Join(researchDir, filename)

	err = os.WriteFile(researchPath, []byte(fullResearch), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Could not save research to %s: %v\n", researchPath, err)
	} else {
		fmt.Fprintf(os.Stderr, "\n✓ Research saved to: %s\n", researchPath)
	}

	return nil
}

func requiresUnsafeConcurrencyFinding(evidence repositoryEvidence) bool {
	for _, content := range evidence.Contents {
		lower := strings.ToLower(content)
		hasMutableMap := strings.Contains(lower, "map[") &&
			(strings.Contains(lower, "sessions[") || strings.Contains(lower, "delete("))
		hasSynchronization := strings.Contains(lower, "sync.mutex") ||
			strings.Contains(lower, "sync.rwmutex") ||
			strings.Contains(lower, ".lock()") ||
			strings.Contains(lower, ".rlock()")
		if hasMutableMap && !hasSynchronization {
			return true
		}
	}
	return false
}

func contradictsUnsafeConcurrencyEvidence(analysis string, evidence repositoryEvidence) bool {
	if !requiresUnsafeConcurrencyFinding(evidence) {
		return false
	}
	lower := strings.ToLower(analysis)
	claimsSafe := strings.Contains(lower, "concurrent access") &&
		(strings.Contains(lower, "is safe") || strings.Contains(lower, "are safe"))
	acknowledgesUnsafe := strings.Contains(lower, "not safe") ||
		strings.Contains(lower, "unsafe") ||
		strings.Contains(lower, "not thread-safe")
	return claimsSafe && !acknowledgesUnsafe
}

type repositoryEvidence struct {
	Language string
	Files    []string
	Contents map[string]string
}

func collectRepositoryEvidence(cwd string) (repositoryEvidence, error) {
	const maxCollectedContentBytes = 64 * 1024
	evidence := repositoryEvidence{
		Language: "Unknown",
		Contents: make(map[string]string),
	}
	collectedContentBytes := 0
	switch {
	case fileExists(filepath.Join(cwd, "go.mod")):
		evidence.Language = "Go"
	case fileExists(filepath.Join(cwd, "mix.exs")):
		evidence.Language = "Elixir"
	case fileExists(filepath.Join(cwd, "Cargo.toml")):
		evidence.Language = "Rust"
	case fileExists(filepath.Join(cwd, "package.json")):
		evidence.Language = "JavaScript/TypeScript"
	case fileExists(filepath.Join(cwd, "pyproject.toml")) || fileExists(filepath.Join(cwd, "requirements.txt")):
		evidence.Language = "Python"
	case fileExists(filepath.Join(cwd, "Gemfile")):
		evidence.Language = "Ruby"
	}

	err := filepath.WalkDir(cwd, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != cwd && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(cwd, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		evidence.Files = append(evidence.Files, relative)
		if shouldIncludeResearchContent(relative) && collectedContentBytes < maxCollectedContentBytes {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			remaining := maxCollectedContentBytes - collectedContentBytes
			if len(content) <= remaining {
				evidence.Contents[relative] = string(content)
				collectedContentBytes += len(content)
			}
		}
		return nil
	})
	sort.Strings(evidence.Files)
	if len(evidence.Files) > 200 {
		evidence.Files = evidence.Files[:200]
	}
	return evidence, err
}

func shouldIncludeResearchContent(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ex", ".exs", ".rs", ".py", ".rb", ".js", ".jsx", ".ts", ".tsx", ".java", ".md":
		return true
	default:
		return path == "go.mod" || path == "Cargo.toml" || path == "Gemfile" || path == "mix.exs"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func buildCodebaseResearchPrompt(question string, evidence repositoryEvidence) string {
	var groundedFiles strings.Builder
	for _, path := range evidence.Files {
		content, ok := evidence.Contents[path]
		if !ok {
			continue
		}
		fmt.Fprintf(&groundedFiles, "\n<file path=%q>\n%s\n</file>\n", path, content)
	}

	return fmt.Sprintf(`Research this repository question:
%s

Deterministic repository evidence:
- Main language detected deterministically: %s
- Files:
%s

Inspected repository contents:
%s

Grounding requirements:
1. Base every technical claim on the inspected contents below and cite exact file paths.
2. Use read_file only if a necessary listed file was omitted from the inspected contents.
3. State the verification command defined by the repository or tests.
4. Do not speculate, use "likely", or mention identifiers absent from the inspected contents.
5. If evidence is insufficient, say exactly what could not be established.
6. Contracts and tests describe expectations; they do not prove the implementation satisfies them.
7. Compare stated expectations with implementation details and report contradictions explicitly.
8. Concurrency claims require synchronization in the implementation (for example locks, atomics, channels, or documented confinement). A WaitGroup or goroutines in a test exercise concurrency; they do not make shared state safe.
9. If mutable shared state has no visible synchronization, state that concurrent access is unsafe and recommend running the repository's race/concurrency verification.

Return concise sections: Findings, Evidence, Verification.`, question, evidence.Language, strings.Join(evidence.Files, "\n"), groundedFiles.String())
}

func extractURLs(text string) []string {
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	return urlRegex.FindAllString(text, -1)
}

func sanitizeFilename(question string) string {
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, question)
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}
	return sanitized
}
