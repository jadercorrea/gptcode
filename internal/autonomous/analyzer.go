package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/config"
	"github.com/jadercorrea/gptcode/internal/llm"
	"github.com/jadercorrea/gptcode/internal/ml"
)

// shouldDeepAnalyze determines if a task needs deep analysis
func shouldDeepAnalyze(task string) bool {
	taskLower := strings.ToLower(task)
	keywords := []string{"bug", "fix", "error", "issue", "crash", "broken", "fails", "incorrect", "unexpected"}
	for _, kw := range keywords {
		if strings.Contains(taskLower, kw) {
			return true
		}
	}
	return false
}

// DeepAnalyze performs deep analysis using a reasoning model
func (a *TaskAnalyzer) DeepAnalyze(ctx context.Context, task string) (string, error) {
	setup, err := config.LoadSetup()
	if err != nil {
		return "", fmt.Errorf("failed to load setup: %w", err)
	}

	selector, err := config.NewModelSelector(setup)
	if err != nil {
		return "", fmt.Errorf("failed to create model selector: %w", err)
	}

	backendName, reasoningModel, err := selector.SelectModel(config.ActionResearch, a.model, "complex")
	if err != nil {
		return "", fmt.Errorf("failed to select reasoning model: %w", err)
	}

	var provider llm.Provider
	backendCfg := setup.Backend[backendName]
	if backendCfg.Type == "ollama" {
		provider = llm.NewOllama(backendCfg.BaseURL)
	} else {
		provider = llm.NewChatCompletion(backendCfg.BaseURL, backendName)
	}

	evidence := repositoryEvidence(a.cwd, extractFileMentions(task), 24*1024)
	deepAnalysisPrompt := fmt.Sprintf(`You are a senior software engineer performing DEEP ANALYSIS of a bug fix task.

TASK:
%s

REPOSITORY EVIDENCE:
%s

INSTRUCTIONS:
1. Base every claim on the repository evidence above. Never invent behavior, files, or root causes.
2. Identify the ROOT CAUSE. If evidence is insufficient, say "UNKNOWN — more repository evidence is required".
3. Determine what files likely need to be modified
4. Identify any edge cases or potential regressions
5. Outline the APPROACH for fixing this bug

Be specific and technical. Focus on the actual code changes needed.

Return your analysis in this format:
ROOT CAUSE: [2-3 sentence explanation of the root cause]
FILES TO MODIFY: [list of likely files]
EDGE CASES: [potential edge cases to consider]
APPROACH: [high-level approach to fix]`, task, evidence)

	response, err := provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a senior software engineer performing deep analysis.",
		UserPrompt:   deepAnalysisPrompt,
		Model:        reasoningModel,
	})

	if err != nil {
		return "", fmt.Errorf("deep analysis failed: %w", err)
	}

	return response.Text, nil
}

func repositoryEvidence(cwd string, paths []string, limit int) string {
	if len(paths) == 0 {
		return "No explicit repository files were identified in the task."
	}
	root, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return "The repository root could not be resolved."
	}
	var evidence strings.Builder
	for _, path := range paths {
		clean := filepath.Clean(path)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, clean))
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		content, err := os.ReadFile(resolved)
		if err != nil {
			continue
		}
		remaining := limit - evidence.Len()
		if remaining <= 0 {
			break
		}
		entry := fmt.Sprintf("\n--- %s ---\n%s\n", clean, content)
		if len(entry) > remaining {
			entry = entry[:remaining]
		}
		evidence.WriteString(entry)
	}
	if evidence.Len() == 0 {
		return "The files named in the task could not be read."
	}
	return evidence.String()
}

// TaskAnalysis represents the result of analyzing a task
type TaskAnalysis struct {
	Intent        string     `json:"intent"`
	Verb          string     `json:"verb"`
	Complexity    int        `json:"complexity"`
	RequiredFiles []string   `json:"required_files"`
	OutputFiles   []string   `json:"output_files"`
	Movements     []Movement `json:"movements,omitempty"`
}

// Movement represents a single phase in a complex task
type Movement struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Goal            string   `json:"goal"`
	Dependencies    []string `json:"dependencies"`
	RequiredFiles   []string `json:"required_files"`
	OutputFiles     []string `json:"output_files"`
	SuccessCriteria []string `json:"success_criteria"`
	Status          string   `json:"status"` // "pending", "executing", "completed", "failed"
}

// TaskAnalyzer analyzes tasks and decomposes them into movements if complex
type TaskAnalyzer struct {
	classifier          *agents.Classifier
	llm                 llm.Provider
	cwd                 string
	model               string
	complexityPredictor *ml.Predictor
}

// NewTaskAnalyzer creates a new task analyzer
func NewTaskAnalyzer(classifier *agents.Classifier, llmProvider llm.Provider, cwd string, model string) *TaskAnalyzer {
	complexityPredictor, err := ml.LoadEmbedded("complexity_detection")
	if err != nil {
		fmt.Printf("Warning: failed to load complexity model: %v\n", err)
	}
	return &TaskAnalyzer{
		classifier:          classifier,
		llm:                 llmProvider,
		cwd:                 cwd,
		model:               model,
		complexityPredictor: complexityPredictor,
	}
}

// Analyze analyzes a task and determines if it needs decomposition
func (a *TaskAnalyzer) Analyze(ctx context.Context, task string) (*TaskAnalysis, error) {
	analysis := &TaskAnalysis{}

	// 0. Deep Analysis for complex tasks - use reasoning model to deeply understand the problem
	if shouldDeepAnalyze(task) {
		deepAnalysis, err := a.DeepAnalyze(ctx, task)
		if err != nil {
			fmt.Printf("[DEEP_ANALYSIS] Warning: deep analysis failed: %v, continuing with standard analysis\n", err)
		} else if deepAnalysis != "" {
			fmt.Printf("[DEEP_ANALYSIS] Completed: %s\n", strings.ReplaceAll(strings.ReplaceAll(deepAnalysis[:min(200, len(deepAnalysis))], "\n", " "), "\t", " "))
			// Prepend deep analysis context to task for better understanding
			task = fmt.Sprintf("DEEP ANALYSIS CONTEXT:\n%s\n\n---\n\nORIGINAL TASK:\n%s", deepAnalysis, task)
		}
	}

	// 1. Use existing classifier for intent
	intent, err := a.classifier.ClassifyIntent(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to classify intent: %w", err)
	}
	analysis.Intent = string(intent)

	// 2. Extract verb and files
	analysis.Verb = extractVerb(task)
	analysis.RequiredFiles = extractFileMentions(task)

	// 3. Estimate complexity (1-10)
	complexity, err := a.estimateComplexity(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to estimate complexity: %w", err)
	}
	analysis.Complexity = complexity

	// 4. If complex (>= 6), decompose into movements
	if complexity >= 6 {
		movements, err := a.decomposeIntoMovements(ctx, task, analysis)
		if err != nil {
			return nil, fmt.Errorf("failed to decompose into movements: %w", err)
		}
		analysis.Movements = movements
	}

	return analysis, nil
}

// extractVerb extracts the primary verb from a task
func extractVerb(task string) string {
	task = strings.ToLower(task)

	verbs := []string{
		"create", "add", "remove", "delete", "update", "modify",
		"refactor", "reorganize", "unify", "split", "merge",
		"read", "list", "show", "explain", "analyze",
	}

	for _, verb := range verbs {
		if strings.Contains(task, verb) {
			return verb
		}
	}

	return "unknown"
}

// extractFileMentions extracts explicit file paths from the task
func extractFileMentions(task string) []string {
	// Match patterns like:
	// - docs/_posts/file.md
	// - src/main.go
	// - /absolute/path.txt
	// Order matters: longer extensions first to avoid .js matching before .json
	filePattern := regexp.MustCompile(`[a-zA-Z0-9_\-./]+\.(json|yaml|yml|md|go|js|ts|py|txt|html|css)`)
	matches := filePattern.FindAllString(task, -1)

	// Deduplicate
	seen := make(map[string]bool)
	var files []string
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			files = append(files, match)
		}
	}

	return files
}

// estimateComplexity uses LLM to score task complexity 1-10
func (a *TaskAnalyzer) estimateComplexity(ctx context.Context, task string) (int, error) {
	if a.complexityPredictor == nil {
		return 5, fmt.Errorf("complexity predictor not loaded")
	}

	class, probs := a.complexityPredictor.Predict(task)
	fmt.Printf("   ML Class: %s (probs: simple=%.2f complex=%.2f multistep=%.2f)\n",
		class, probs["simple"], probs["complex"], probs["multistep"])

	var score int
	switch class {
	case "simple":
		score = 3
	case "complex":
		score = 7
	case "multistep":
		score = 8
	default:
		score = 5
	}

	return score, nil
}

// decomposeIntoMovements breaks a complex task into movements
func (a *TaskAnalyzer) decomposeIntoMovements(ctx context.Context, task string, analysis *TaskAnalysis) ([]Movement, error) {
	prompt := fmt.Sprintf(`Decompose this complex task into 2-5 independent movements (phases).

Task: %s
Intent: %s
Verb: %s

Rules:
- Each movement should be independently executable
- Define clear dependencies (Movement B depends on Movement A)
- Each movement should have 1-3 success criteria
- Movements should be sequential (not parallel)
- Be specific about files to read/create
- AVOID creating intermediate/temporary files - process data in memory when possible
- Only create files that are part of the final task goal
- For query/research intents: movements should RETRIEVE and DISPLAY information, NOT create files
- For query tasks: use goals like "Retrieve", "Display", "Show", "Analyze" - NEVER "Create", "Write", "Generate file"

Example:
Task: "Use gh CLI to review and publish blog posts from open PRs in docs/_posts"
Response:
[
  {
    "id": "movement-1",
    "name": "Get PR Content",
    "description": "Use gh CLI to fetch file changes from open PRs",
    "goal": "Retrieve blog post content from PR diffs",
    "dependencies": [],
    "required_files": [],
    "output_files": [],
    "success_criteria": [
      "gh pr list executes successfully",
      "PR content is retrieved"
    ]
  },
  {
    "id": "movement-2",
    "name": "Review and Publish",
    "description": "Review posts format, update date, publish to docs/_posts/",
    "goal": "Create properly formatted blog posts with correct dates",
    "dependencies": ["movement-1"],
    "required_files": ["docs/_posts/*.md"],
    "output_files": ["docs/_posts/2025-12-03-new-post.md"],
    "success_criteria": [
      "posts match existing format in docs/_posts",
      "filenames use next available date",
      "posts are valid markdown"
    ]
  }
]

Return ONLY valid JSON array of movements, no explanation.`, task, analysis.Intent, analysis.Verb)

	// Try with configured model first
	resp, err := a.llm.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a task decomposition assistant. Return only valid JSON.",
		UserPrompt:   prompt,
		Model:        a.model,
	})
	if err != nil {
		return nil, err
	}

	// Parse JSON
	var movements []Movement
	responseText := strings.TrimSpace(resp.Text)

	// Remove markdown code blocks if present
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	err = json.Unmarshal([]byte(responseText), &movements)
	if err != nil {
		return nil, fmt.Errorf("failed to parse movements JSON: %w\nResponse: %s", err, responseText)
	}

	// Validate that movements is not empty
	if len(movements) == 0 {
		return nil, fmt.Errorf("model returned empty movements array - decomposition failed")
	}

	// Initialize status
	for i := range movements {
		movements[i].Status = "pending"
	}

	return movements, nil
}
