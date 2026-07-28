package modes

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResearchEvidenceFilters(t *testing.T) {
	for _, directory := range []string{".git", ".gptcode", "build", "dist", "node_modules", "vendor"} {
		if !shouldSkipResearchDirectory(directory) {
			t.Errorf("should skip directory %q", directory)
		}
	}
	if shouldSkipResearchDirectory("internal") {
		t.Error("should inspect source directories")
	}

	for _, file := range []string{".env", ".env.production", ".env.example"} {
		if !shouldSkipResearchFile(file) {
			t.Errorf("should skip secret file %q", file)
		}
	}

	for _, path := range []string{"main.go", "lib/app.ex", "README.md", "go.mod", "Cargo.toml", "Gemfile", "mix.exs"} {
		if !shouldIncludeResearchContent(path) {
			t.Errorf("should include research content %q", path)
		}
	}
	if shouldIncludeResearchContent("logo.png") {
		t.Error("should not load binary assets into research evidence")
	}
}

func TestSanitizeFilenameHasStableFallback(t *testing.T) {
	tests := map[string]string{
		"How Does Auth Work?": "how-does-auth-work",
		"???":                 "research",
	}
	for input, expected := range tests {
		if actual := sanitizeFilename(input); actual != expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestExtractURLsTrimsSentencePunctuation(t *testing.T) {
	actual := extractURLs("See https://example.com/docs). Then https://go.dev/test, please.")
	expected := []string{"https://example.com/docs", "https://go.dev/test"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("extractURLs() = %#v, want %#v", actual, expected)
	}
}

func TestBuildCodebaseResearchPromptRequiresGroundedEvidence(t *testing.T) {
	prompt := buildCodebaseResearchPrompt(
		"Explain session expiration and concurrent access",
		repositoryEvidence{
			Language: "Go",
			Files:    []string{"README.md", "go.mod", "session/store.go", "session/store_test.go"},
			Contents: map[string]string{
				"session/store.go": "func (s *Store) Active(token string, now time.Time) bool",
			},
		},
	)

	for _, required := range []string{
		"Main language detected deterministically: Go",
		"session/store.go",
		"session/store_test.go",
		"func (s *Store) Active",
		`1 | func (s *Store) Active`,
		"read_file",
		"exact file paths",
		"Do not speculate",
		"verification command",
		"Contracts and tests describe expectations",
		"Behavioral conclusions require implementation evidence",
		"report contradictions",
		"Concurrency claims require synchronization in the implementation",
		"WaitGroup or goroutines in a test exercise concurrency",
		"internally consistent",
		"Do not claim a command passed",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("research prompt missing %q:\n%s", required, prompt)
		}
	}
}

func TestCollectRepositoryEvidencePrioritizesQuestionRelevantImplementation(t *testing.T) {
	repository := t.TempDir()
	writeTestFile(t, filepath.Join(repository, "go.mod"), "module example.com/repository\n")
	writeTestFile(t, filepath.Join(repository, "README.md"), strings.Repeat("documented contract\n", 4_000))
	writeTestFile(t, filepath.Join(repository, "internal", "unrelated", "large.go"), "package unrelated\n"+strings.Repeat("// filler\n", 8_000))
	target := filepath.Join(repository, "internal", "tools", "tools.go")
	writeTestFile(t, target, "package tools\nfunc readFile() { /* implementation evidence */ }\n")

	evidence, err := collectRepositoryEvidence(
		repository,
		"Can model-invoked file tools escape the repository?",
	)
	if err != nil {
		t.Fatalf("collect repository evidence: %v", err)
	}

	content, ok := evidence.Contents["internal/tools/tools.go"]
	if !ok {
		t.Fatal("question-relevant implementation was omitted from bounded evidence")
	}
	if !strings.Contains(content, "implementation evidence") {
		t.Fatalf("unexpected relevant evidence: %q", content)
	}
}

func TestResearchDetectsUnsupportedConcurrencySafetyClaim(t *testing.T) {
	evidence := repositoryEvidence{
		Files: []string{"session/store.go"},
		Contents: map[string]string{
			"session/store.go": "type Store struct { sessions map[string]Session }\nfunc (s *Store) Put(v Session) { s.sessions[v.Token] = v }",
		},
	}

	if !requiresUnsafeConcurrencyFinding(evidence) {
		t.Fatal("expected mutable unsynchronized map evidence to require an unsafe finding")
	}
	if !contradictsUnsafeConcurrencyEvidence("Concurrent access to Store is safe.", evidence) {
		t.Fatal("expected unsupported safety claim to be rejected")
	}
	if contradictsUnsafeConcurrencyEvidence("Concurrent access is not safe because the map has no lock.", evidence) {
		t.Fatal("a grounded unsafe finding must be accepted")
	}
}

func TestResearchDetectsUnsupportedGoMethodLockClaim(t *testing.T) {
	evidence := repositoryEvidence{
		Language: "Go",
		Contents: map[string]string{
			"cache.go": `package cache
func (c *Cache) Get() {
	c.mu.RLock()
	defer c.mu.RUnlock()
}
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return 0
}`,
		},
	}

	issues := unsupportedGoLockClaims(
		"Read operations (`Get`, `Len`) acquire read locks (`RLock`).",
		evidence,
	)
	if len(issues) != 1 || !strings.Contains(issues[0], "Len") {
		t.Fatalf("expected the false Len lock claim to be rejected, got %v", issues)
	}
	if issues := unsupportedGoLockClaims(
		"`Get` starts with RLock; `Len` takes the exclusive Lock because it deletes expired entries.",
		evidence,
	); len(issues) != 0 {
		t.Fatalf("expected exact per-method lock claims to pass, got %v", issues)
	}
}

func TestResearchRejectsCIClaimsWithoutCIEvidence(t *testing.T) {
	evidence := repositoryEvidence{
		Files: []string{"cache.go", "cache_test.go", "task.md"},
	}
	if issue := unsupportedCIClaim("These commands are executed by the repository's CI system.", evidence); issue == "" {
		t.Fatal("expected unsupported CI claim to be rejected")
	}

	evidence.Files = append(evidence.Files, ".github/workflows/verify.yml")
	if issue := unsupportedCIClaim("These commands are executed by CI.", evidence); issue != "" {
		t.Fatalf("expected CI claim with workflow evidence to pass, got %q", issue)
	}
}

func TestCollectRepositoryEvidenceStaysInsidePublicRepositoryContext(t *testing.T) {
	repository := t.TempDir()
	outside := t.TempDir()

	writeTestFile(t, filepath.Join(repository, "go.mod"), "module example.com/repository\n")
	writeTestFile(t, filepath.Join(repository, "main.go"), "package main\n")
	writeTestFile(t, filepath.Join(repository, ".gptcode", "private.go"), "package private\n")
	writeTestFile(t, filepath.Join(repository, "dist", "generated.go"), "package generated\n")
	writeTestFile(t, filepath.Join(outside, "secret.go"), "package secret\n")

	if err := os.Symlink(
		filepath.Join(outside, "secret.go"),
		filepath.Join(repository, "linked-secret.go"),
	); err != nil {
		t.Fatalf("create fixture symlink: %v", err)
	}

	evidence, err := collectRepositoryEvidence(repository)
	if err != nil {
		t.Fatalf("collect repository evidence: %v", err)
	}

	if evidence.Language != "Go" {
		t.Fatalf("language = %q, want Go", evidence.Language)
	}
	for _, rejected := range []string{
		".gptcode/private.go",
		"dist/generated.go",
		"linked-secret.go",
	} {
		if containsString(evidence.Files, rejected) {
			t.Errorf("evidence must not list %q", rejected)
		}
		if _, ok := evidence.Contents[rejected]; ok {
			t.Errorf("evidence must not read %q", rejected)
		}
	}
	if !containsString(evidence.Files, "main.go") {
		t.Fatal("evidence must include repository source files")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
