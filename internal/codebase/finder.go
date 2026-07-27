package codebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/langdetect"
	"github.com/jadercorrea/gptcode/internal/llm"
)

type FileFinder struct {
	workDir    string
	queryAgent *agents.QueryAgent
	language   string
}

type RelevantFile struct {
	Path       string
	Reason     string
	Confidence float64
}

func NewFileFinder(provider llm.Provider, workDir, queryModel string) (*FileFinder, error) {
	lang := langdetect.DetectLanguage(workDir)
	if lang == langdetect.Unknown {
		lang = "unknown"
	}

	queryAgent := agents.NewQuery(provider, workDir, queryModel)

	return &FileFinder{
		workDir:    workDir,
		queryAgent: queryAgent,
		language:   string(lang),
	}, nil
}

func (f *FileFinder) FindRelevantFiles(ctx context.Context, issueDescription string) ([]RelevantFile, error) {
	prompt := fmt.Sprintf(`Given this issue:

%s

Identify the 3-5 most relevant files to modify.

Use list_files and read_file tools to explore the codebase.
Focus on:
1. Main implementation files
2. Related test files
3. Configuration files if needed

For each file, provide:
- Exact file path
- Why it's relevant
- Confidence (high/medium/low)

Format your response as:
FILE: path/to/file.ext
REASON: Brief explanation
CONFIDENCE: high|medium|low

---`, issueDescription)

	response, err := f.queryAgent.Execute(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to find relevant files: %w", err)
	}

	files := parseFileRecommendations(response)
	if len(files) == 0 {
		return f.fallbackSearch(issueDescription)
	}

	return files, nil
}

func (f *FileFinder) IdentifyTestFiles(ctx context.Context, implementationFile string) ([]string, error) {
	prompt := fmt.Sprintf(`Given implementation file: %s

Find corresponding test files using list_files.

Language: %s

Common patterns:
- Go: *_test.go in same dir
- TypeScript: *.test.ts, *.spec.ts
- Python: test_*.py, *_test.py
- Elixir: *_test.exs
- Ruby: *_spec.rb

List full paths only, one per line.`, implementationFile, f.language)

	response, err := f.queryAgent.Execute(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to identify test files: %w", err)
	}

	lines := strings.Split(response, "\n")
	var testFiles []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, err := os.Stat(filepath.Join(f.workDir, line)); err == nil {
			testFiles = append(testFiles, line)
		}
	}

	return testFiles, nil
}

func (f *FileFinder) AnalyzeGitHistory(ctx context.Context, issueKeywords []string) ([]string, error) {
	prompt := fmt.Sprintf(`Analyze git history to find similar past changes.

Keywords: %s

Use git_log tool to search for relevant commits.
Look for commits that:
1. Fixed similar issues
2. Modified related components
3. Added similar features

Return list of file paths that were commonly modified, one per line.`, strings.Join(issueKeywords, ", "))

	response, err := f.queryAgent.Execute(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(response, "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			files = append(files, line)
		}
	}

	return files, nil
}

func parseFileRecommendations(response string) []RelevantFile {
	var files []RelevantFile
	lines := strings.Split(response, "\n")

	var currentFile RelevantFile
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "FILE:") {
			if currentFile.Path != "" {
				files = append(files, currentFile)
			}
			currentFile = RelevantFile{
				Path: strings.TrimSpace(strings.TrimPrefix(line, "FILE:")),
			}
		} else if strings.HasPrefix(line, "REASON:") {
			currentFile.Reason = strings.TrimSpace(strings.TrimPrefix(line, "REASON:"))
		} else if strings.HasPrefix(line, "CONFIDENCE:") {
			conf := strings.TrimSpace(strings.TrimPrefix(line, "CONFIDENCE:"))
			switch strings.ToLower(conf) {
			case "high":
				currentFile.Confidence = 0.9
			case "medium":
				currentFile.Confidence = 0.6
			case "low":
				currentFile.Confidence = 0.3
			}
		}
	}

	if currentFile.Path != "" {
		files = append(files, currentFile)
	}

	return files
}

func (f *FileFinder) fallbackSearch(issueDescription string) ([]RelevantFile, error) {
	keywords := extractKeywords(issueDescription)

	var files []RelevantFile
	for _, keyword := range keywords {
		pattern := fmt.Sprintf("*%s*.%s", keyword, getExtension(f.language))
		matches, err := filepath.Glob(filepath.Join(f.workDir, "**", pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			relPath, _ := filepath.Rel(f.workDir, match)
			files = append(files, RelevantFile{
				Path:       relPath,
				Reason:     fmt.Sprintf("Filename matches keyword: %s", keyword),
				Confidence: 0.5,
			})

			if len(files) >= 5 {
				break
			}
		}
	}

	return files, nil
}

func extractKeywords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var keywords []string

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
		"are": true, "was": true, "were": true, "be": true, "been": true,
	}

	for _, word := range words {
		if len(word) > 3 && !stopWords[word] {
			keywords = append(keywords, word)
			if len(keywords) >= 5 {
				break
			}
		}
	}

	return keywords
}

func getExtension(language string) string {
	switch strings.ToLower(language) {
	case "go":
		return "go"
	case "typescript", "javascript":
		return "ts"
	case "python":
		return "py"
	case "elixir":
		return "ex"
	case "ruby":
		return "rb"
	default:
		return "*"
	}
}
