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

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/output"

	"golang.org/x/term"
)

const localModelTimeout = 2 * time.Minute

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

	queryModel := configuredAgentModel(backendCfg, setup.Defaults.Profile, "research")
	queryAgent := agents.NewQuery(customExec, cwd, queryModel)

	evidence, err := collectRepositoryEvidence(cwd, question)
	if err != nil {
		return fmt.Errorf("collect repository evidence: %w", err)
	}
	codebasePrompt := buildCodebaseResearchPrompt(question, evidence)
	researchCtx, cancel := context.WithTimeout(context.Background(), localModelTimeout)
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

	for correctionAttempt := 0; correctionAttempt < 2; correctionAttempt++ {
		issues := unsupportedGoLockClaims(codebaseAnalysis, evidence)
		if ciIssue := unsupportedCIClaim(codebaseAnalysis, evidence); ciIssue != "" {
			issues = append(issues, ciIssue)
		}
		if len(issues) == 0 {
			break
		}
		correctionCtx, correctionCancel := context.WithTimeout(context.Background(), localModelTimeout)
		corrected, correctionErr := queryAgent.Execute(correctionCtx, []llm.ChatMessage{{
			Role: "user",
			Content: buildCodebaseResearchPrompt(question, evidence) + fmt.Sprintf(`

Mandatory correction:
The draft made these claims that contradict the inspected repository evidence:
- %s

Rewrite the answer from scratch. Describe the exact lock acquired by each
method separately. Return only Findings, Evidence, and Verification; do not
include the previous draft or a corrections appendix.`, strings.Join(issues, "\n- ")),
		}}, nil)
		correctionCancel()
		if correctionErr != nil {
			return fmt.Errorf("correct unsupported lock claims: %w", correctionErr)
		}
		codebaseAnalysis = corrected
	}
	remainingIssues := unsupportedGoLockClaims(codebaseAnalysis, evidence)
	if ciIssue := unsupportedCIClaim(codebaseAnalysis, evidence); ciIssue != "" {
		remainingIssues = append(remainingIssues, ciIssue)
	}
	if len(remainingIssues) > 0 {
		return fmt.Errorf("codebase analysis contradicted deterministic evidence: %s", strings.Join(remainingIssues, "; "))
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

func unsupportedGoLockClaims(analysis string, evidence repositoryEvidence) []string {
	if !strings.EqualFold(evidence.Language, "Go") {
		return nil
	}

	methodUsesReadLock := make(map[string]bool)
	methodPattern := regexp.MustCompile(`(?s)func\s+\([^)]*\)\s+([A-Za-z][A-Za-z0-9_]*)\([^)]*\)[^{]*\{(.*?)\n\}`)
	for _, content := range evidence.Contents {
		for _, match := range methodPattern.FindAllStringSubmatch(content, -1) {
			if len(match) == 3 {
				methodUsesReadLock[match[1]] = strings.Contains(match[2], ".RLock()")
			}
		}
	}

	var issues []string
	segments := strings.FieldsFunc(analysis, func(r rune) bool {
		return r == '\n' || r == ';'
	})
	for _, segment := range segments {
		lower := strings.ToLower(segment)
		if !strings.Contains(lower, "read lock") && !strings.Contains(lower, "rlock") {
			continue
		}
		for method, usesReadLock := range methodUsesReadLock {
			if usesReadLock {
				continue
			}
			methodPattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(method) + `\b`)
			if methodPattern.MatchString(segment) {
				issues = append(issues, fmt.Sprintf("%s does not acquire RLock, but the draft says: %s", method, strings.TrimSpace(segment)))
			}
		}
	}
	sort.Strings(issues)
	return issues
}

func unsupportedCIClaim(analysis string, evidence repositoryEvidence) string {
	mentionsCI := regexp.MustCompile(`(?i)\bCI(?:/CD)?\b`).MatchString(analysis)
	if !mentionsCI {
		return ""
	}
	for _, path := range evidence.Files {
		lower := strings.ToLower(path)
		if strings.HasPrefix(lower, ".github/workflows/") ||
			strings.Contains(lower, "gitlab-ci") ||
			strings.Contains(lower, "circleci") ||
			strings.Contains(lower, "buildkite") {
			return ""
		}
	}
	return "the draft claims CI execution, but no CI configuration exists in the inspected evidence"
}

type repositoryEvidence struct {
	Language string
	Files    []string
	Contents map[string]string
}

func collectRepositoryEvidence(cwd string, questions ...string) (repositoryEvidence, error) {
	const maxCollectedContentBytes = 64 * 1024
	evidence := repositoryEvidence{
		Language: "Unknown",
		Contents: make(map[string]string),
	}
	question := strings.Join(questions, " ")
	var contentCandidates []string
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
			if path != cwd && shouldSkipResearchDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || shouldSkipResearchFile(entry.Name()) {
			return nil
		}
		relative, err := filepath.Rel(cwd, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		evidence.Files = append(evidence.Files, relative)
		if shouldIncludeResearchContent(relative) {
			contentCandidates = append(contentCandidates, relative)
		}
		return nil
	})
	if err != nil {
		return evidence, err
	}

	sortResearchPaths(evidence.Files, question)
	if len(evidence.Files) > 200 {
		evidence.Files = evidence.Files[:200]
	}
	sortResearchPaths(contentCandidates, question)

	collectedContentBytes := 0
	for _, relative := range contentCandidates {
		if collectedContentBytes >= maxCollectedContentBytes {
			break
		}
		content, readErr := os.ReadFile(filepath.Join(cwd, filepath.FromSlash(relative)))
		if readErr != nil {
			return evidence, readErr
		}
		remaining := maxCollectedContentBytes - collectedContentBytes
		if len(content) > remaining {
			continue
		}
		evidence.Contents[relative] = string(content)
		collectedContentBytes += len(content)
	}

	return evidence, nil
}

func sortResearchPaths(paths []string, question string) {
	sort.SliceStable(paths, func(i, j int) bool {
		leftScore := researchPathScore(paths[i], question)
		rightScore := researchPathScore(paths[j], question)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return paths[i] < paths[j]
	})
}

func researchPathScore(path, question string) int {
	lowerPath := strings.ToLower(path)
	score := 0
	for _, term := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if len(term) >= 3 && strings.Contains(lowerPath, term) {
			score += 10
		}
	}
	if isImplementationResearchPath(path) {
		score++
	}
	return score
}

func isImplementationResearchPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".ex", ".exs", ".rs", ".py", ".rb", ".js", ".jsx", ".ts", ".tsx", ".java":
		return true
	default:
		return false
	}
}

func shouldSkipResearchDirectory(name string) bool {
	switch name {
	case ".git", ".gptcode", ".idea", ".vscode",
		"bin", "build", "coverage", "dist", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func shouldSkipResearchFile(name string) bool {
	return name == ".env" || strings.HasPrefix(name, ".env.")
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
		fmt.Fprintf(&groundedFiles, "\n<file path=%q>\n%s\n</file>\n", path, numberLines(content))
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
7. Behavioral conclusions require implementation evidence. Documentation, plans, and quality claims are not implementation evidence.
8. Compare stated expectations with implementation details and report contradictions explicitly.
9. Concurrency claims require synchronization in the implementation (for example locks, atomics, channels, or documented confinement). A WaitGroup or goroutines in a test exercise concurrency; they do not make shared state safe.
10. If mutable shared state has no visible synchronization, state that concurrent access is unsafe and recommend running the repository's race/concurrency verification.
11. Make the answer internally consistent. Describe the lock actually acquired by each method; do not classify a method as a read operation if its implementation takes a write lock or mutates state.
12. Do not claim a command passed unless its exit status and output are included in the evidence. Distinguish commands required by the repository from commands actually executed.

Return concise sections: Findings, Evidence, Verification.`, question, evidence.Language, strings.Join(evidence.Files, "\n"), groundedFiles.String())
}

func extractURLs(text string) []string {
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	urls := urlRegex.FindAllString(text, -1)
	for index, url := range urls {
		urls[index] = strings.TrimRight(url, ".,;:!?)]}")
	}
	return urls
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
	if sanitized == "" {
		return "research"
	}
	return sanitized
}
