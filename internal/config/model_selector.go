package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadercorrea/gptcode/internal/catalog"
)

type ActionType string

const (
	ActionEdit     ActionType = "edit"
	ActionReview   ActionType = "review"
	ActionPlan     ActionType = "plan"
	ActionResearch ActionType = "research"
	ActionRoute    ActionType = "route"
)

type ModelCapabilities struct {
	SupportsTools          bool   `json:"supports_tools"`
	SupportsFileOperations bool   `json:"supports_file_operations"`
	SupportsCodeExecution  bool   `json:"supports_code_execution"`
	Notes                  string `json:"notes"`
}

type ModelInfo struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	CostPer1M      float64           `json:"cost_per_1m"`
	RateLimitDaily int               `json:"rate_limit_daily"`
	ContextWindow  int               `json:"context_window"`
	TokensPerSec   int               `json:"tokens_per_sec"`
	Capabilities   ModelCapabilities `json:"capabilities"`
	RecommendedFor []string          `json:"recommended_for"` // e.g. ["editor", "query", "router"]
	Backend        string
}

type ModelFeedback struct {
	ModelID    string     `json:"model_id"`
	Action     ActionType `json:"action"`
	Language   string     `json:"language"`
	Success    bool       `json:"success"`
	Complexity string     `json:"complexity"` // simple, complex, multistep
}

type ModelUsage struct {
	Requests     int    `json:"requests"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	CachedTokens int    `json:"cached_tokens"`
	LastError    string `json:"last_error,omitempty"`
}

type ModelSelector struct {
	catalog     map[string][]ModelInfo
	feedback    []ModelFeedback
	usage       map[string]map[string]ModelUsage
	setup       *Setup
	recommender *RecommenderModel
}

func NewModelSelector(setup *Setup) (*ModelSelector, error) {
	selector := &ModelSelector{
		catalog:  make(map[string][]ModelInfo),
		feedback: []ModelFeedback{},
		usage:    make(map[string]map[string]ModelUsage),
		setup:    setup,
	}

	if err := selector.loadCatalog(); err != nil {
		return nil, fmt.Errorf("failed to load catalog: %w", err)
	}

	if err := selector.loadFeedback(); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Could not load feedback: %v\n", err)
	}

	if err := selector.loadUsage(); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Could not load usage: %v\n", err)
	}

	workDir, _ := os.Getwd()
	if recommender, err := LoadRecommender(workDir); err == nil {
		selector.recommender = recommender
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Loaded ML recommender\n")
		}
	} else if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] No ML recommender: %v\n", err)
	}

	return selector, nil
}

func (ms *ModelSelector) loadCatalog() error {
	catalogPath := filepath.Join(configDir(), "models_catalog.json")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		data = catalog.GetDefaultModels()
		if len(data) == 0 {
			return fmt.Errorf("user catalog not found and embedded catalog is empty")
		}
	}

	var rawCatalog map[string]interface{}
	if err := json.Unmarshal(data, &rawCatalog); err != nil {
		return err
	}

	for backend, backendData := range rawCatalog {
		backendMap, ok := backendData.(map[string]interface{})
		if !ok {
			continue
		}

		modelsData, ok := backendMap["models"].([]interface{})
		if !ok {
			continue
		}

		for _, modelData := range modelsData {
			modelMap, ok := modelData.(map[string]interface{})
			if !ok {
				continue
			}

			model := ModelInfo{
				Backend: backend,
			}

			if id, ok := modelMap["id"].(string); ok {
				model.ID = id
			}
			if name, ok := modelMap["name"].(string); ok {
				model.Name = name
			}
			if cost, ok := modelMap["cost_per_1m"].(float64); ok {
				model.CostPer1M = cost
			}
			if limit, ok := modelMap["rate_limit_daily"].(float64); ok {
				model.RateLimitDaily = int(limit)
			}
			if ctx, ok := modelMap["context_window"].(float64); ok {
				model.ContextWindow = int(ctx)
			}
			if tps, ok := modelMap["tokens_per_sec"].(float64); ok {
				model.TokensPerSec = int(tps)
			}

			// CRITICAL: Default to false - models must explicitly declare tool support
			// Small models (8B and below) typically don't support tool calling well
			model.Capabilities.SupportsTools = false
			model.Capabilities.SupportsFileOperations = false
			model.Capabilities.SupportsCodeExecution = false

			// Auto-detect capabilities based on model characteristics if not explicitly set
			modelIDLower := strings.ToLower(model.ID)
			modelNameLower := strings.ToLower(model.Name)
			// Known model families that support tools
			toolCapableModels := []string{"gpt-4", "gpt-4o", "gpt-4.5", "gpt-5", "claude-3", "claude-4", "sonnet", "opus", "haiku", "gemini-2", "gemini-2.5", "sonar", "deepseek-chat", "llama-3.1", "llama-3.2", "llama-3.3", "mistral-large", "mistral-small", "mixtral", "qwen-2.5"}
			for _, toolModel := range toolCapableModels {
				if strings.Contains(modelIDLower, toolModel) || strings.Contains(modelNameLower, toolModel) {
					model.Capabilities.SupportsTools = true
					model.Capabilities.SupportsFileOperations = true
					break
				}
			}

			// Special case: OpenRouter free models typically lack tool support
			if strings.Contains(modelIDLower, ":free") || strings.Contains(modelIDLower, "-free") || strings.Contains(modelNameLower, " free") {
				model.Capabilities.SupportsTools = false
				model.Capabilities.SupportsFileOperations = false
			}

			if caps, ok := modelMap["capabilities"].(map[string]interface{}); ok {
				if val, ok := caps["supports_tools"].(bool); ok {
					model.Capabilities.SupportsTools = val
				}
				if val, ok := caps["supports_file_operations"].(bool); ok {
					model.Capabilities.SupportsFileOperations = val
				}
				if val, ok := caps["supports_code_execution"].(bool); ok {
					model.Capabilities.SupportsCodeExecution = val
				}
				if val, ok := caps["notes"].(string); ok {
					model.Capabilities.Notes = val
				}
			}

			// Load recommended_for tags from catalog
			if recFor, ok := modelMap["recommended_for"].([]interface{}); ok {
				for _, r := range recFor {
					if s, ok := r.(string); ok {
						model.RecommendedFor = append(model.RecommendedFor, s)
					}
				}
			}

			ms.catalog[backend] = append(ms.catalog[backend], model)
		}
	}

	return nil
}

func (ms *ModelSelector) loadFeedback() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	feedbackDir := filepath.Join(home, ".gptcode", "feedback")
	entries, err := os.ReadDir(feedbackDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(feedbackDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var rawEvents []map[string]interface{}
		if err := json.Unmarshal(data, &rawEvents); err != nil {
			continue
		}

		for _, event := range rawEvents {
			fb := ms.convertFeedbackEvent(event)
			if fb.ModelID != "" && fb.Action != "" {
				ms.feedback = append(ms.feedback, fb)
			}
		}
	}

	return nil
}

func (ms *ModelSelector) convertFeedbackEvent(event map[string]interface{}) ModelFeedback {
	fb := ModelFeedback{}

	if val, ok := event["model"].(string); ok {
		fb.ModelID = val
	}

	if agent, ok := event["agent"].(string); ok {
		switch strings.ToLower(agent) {
		case "editor":
			fb.Action = ActionEdit
		case "reviewer", "validator":
			fb.Action = ActionReview
		case "planner":
			fb.Action = ActionPlan
		case "research":
			fb.Action = ActionResearch
		}
	}

	if sentiment, ok := event["sentiment"].(string); ok {
		fb.Success = sentiment == "good"
	}

	fb.Language = "unknown"
	if task, ok := event["task"].(string); ok {
		taskLower := strings.ToLower(task)
		if strings.Contains(taskLower, ".go") {
			fb.Language = "go"
		} else if strings.Contains(taskLower, ".py") {
			fb.Language = "python"
		} else if strings.Contains(taskLower, ".ts") || strings.Contains(taskLower, ".js") {
			fb.Language = "typescript"
		} else if strings.Contains(taskLower, ".ex") || strings.Contains(taskLower, ".exs") {
			fb.Language = "elixir"
		}

		fb.Complexity = "simple"
		if strings.Contains(taskLower, "refactor") ||
			strings.Contains(taskLower, "reorganize") ||
			strings.Contains(taskLower, "complex") {
			fb.Complexity = "complex"
		}
	}

	return fb
}

func (ms *ModelSelector) loadUsage() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	usagePath := filepath.Join(home, ".gptcode", "usage.json")
	data, err := os.ReadFile(usagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &ms.usage); err != nil {
		return err
	}

	return nil
}

func (ms *ModelSelector) saveUsage() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	usagePath := filepath.Join(home, ".gptcode", "usage.json")
	data, err := json.MarshalIndent(ms.usage, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(usagePath, data, 0644)
}

func (ms *ModelSelector) RecordUsage(backend, model string, success bool, errorMsg string) {
	ms.RecordUsageWithTokens(backend, model, success, errorMsg, 0, 0, 0)
}

func (ms *ModelSelector) RecordUsageWithTokens(backend, model string, success bool, errorMsg string, inputTokens, outputTokens, cachedTokens int) {
	today := time.Now().Format("2006-01-02")
	if ms.usage[today] == nil {
		ms.usage[today] = make(map[string]ModelUsage)
	}

	key := backend + "/" + model
	usage := ms.usage[today][key]
	usage.Requests++
	usage.InputTokens += inputTokens
	usage.OutputTokens += outputTokens
	usage.CachedTokens += cachedTokens
	if !success {
		usage.LastError = errorMsg
	}
	ms.usage[today][key] = usage

	if err := ms.saveUsage(); err != nil && os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[WARN] Failed to save usage: %v\n", err)
	}
}

func (ms *ModelSelector) getTodayUsage(backend, model string) ModelUsage {
	today := time.Now().Format("2006-01-02")
	if ms.usage[today] == nil {
		return ModelUsage{}
	}

	key := backend + "/" + model
	return ms.usage[today][key]
}

func (ms *ModelSelector) SelectModel(action ActionType, language string, complexity string) (backend string, model string, err error) {
	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] SelectModel called: action=%s lang=%s complexity=%s\n",
			action, language, complexity)
	}

	mode := ms.setup.Defaults.Mode
	defaultBackend := ms.setup.Defaults.Backend

	// An explicit model in the active backend/profile is the user's routing
	// decision. Approved models are fallbacks and must not silently move a
	// local execution to a cloud backend.
	if backendCfg, ok := ms.setup.Backend[defaultBackend]; ok {
		configuredModel := explicitlyConfiguredModelForAction(backendCfg, ms.setup.Defaults.Profile, action)
		if configuredModel != "" {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Using configured model for action=%s: %s/%s\n",
					action, defaultBackend, configuredModel)
			}
			return defaultBackend, configuredModel, nil
		}
	}

	// First, try approved models for this action
	approvedModels := ms.setup.GetApprovedModelsForAction(string(action))
	if len(approvedModels) > 0 {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Trying %d approved models for action=%s\n", len(approvedModels), action)
		}
		for _, approved := range approvedModels {
			backend, model, err := ms.trySelectApprovedModel(approved.Model, action, language, complexity)
			if err == nil && !modeAllowsBackend(mode, backend, ms.setup) {
				err = fmt.Errorf("backend %q is not allowed in %s mode", backend, mode)
			}
			if err == nil {
				if os.Getenv("GPTCODE_DEBUG") == "1" {
					fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Approved model selected: %s/%s\n", backend, model)
				}
				return backend, model, nil
			}
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Approved model %s failed: %v\n", approved.Model, err)
			}
		}
		// All approved models failed — do NOT fall through to scored catalog.
		// The approved list is authoritative; falling back would defeat its purpose.
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] All approved models failed for action=%s, refusing to use catalog fallback\n", action)
		}
		ms.logBlockedNotification(string(action), language)
		return "", "", fmt.Errorf("all approved models failed for action=%s (check setup.yaml and models_catalog.json)", action)
	}

	type scoredModel struct {
		backend string
		model   string
		score   float64
	}
	var scored []scoredModel

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] SelectModel action=%s lang=%s mode=%s defaultBackend=%s\n",
			action, language, mode, defaultBackend)
		fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Catalog has %d backends\n", len(ms.catalog))
	}

	for backend, models := range ms.catalog {
		if mode == "local" && backend != "ollama" {
			continue
		}
		if mode == "cloud" && backend == "ollama" {
			continue
		}

		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Checking backend=%s with %d models\n", backend, len(models))
		}

		for _, modelInfo := range models {
			score := ms.scoreModel(modelInfo, action, language, complexity)
			if score > 0 {
				// Boost score for default backend to prioritize it
				if backend == defaultBackend {
					score += 100
				}
				scored = append(scored, scoredModel{
					backend: backend,
					model:   modelInfo.ID,
					score:   score,
				})
			}
		}
	}

	if len(scored) == 0 {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] No models scored > 0 for action=%s lang=%s\n", action, language)
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Checked %d models in catalog\n", len(ms.catalog[defaultBackend]))
		}

		// Log blocked notification
		ms.logBlockedNotification(string(action), language)

		return "", "", fmt.Errorf("no suitable model found for action=%s lang=%s", action, language)
	}

	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Exploration is opt-in. Production task routing must be deterministic;
	// silently choosing a lower-ranked model makes executions irreproducible.
	best := scored[0]
	if os.Getenv("GPTCODE_MODEL_EXPLORATION") == "1" && len(scored) > 1 && rand.Float64() < 0.10 {
		// Pick randomly from top 5 (or however many we have)
		topN := 5
		if len(scored) < topN {
			topN = len(scored)
		}
		exploreIdx := rand.Intn(topN)
		best = scored[exploreIdx]
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] EXPLORATION: picked %s/%s instead of best\n", best.backend, best.model)
		}
	}

	if os.Getenv("GPTCODE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Action=%s Lang=%s -> %s/%s (score=%.2f)\n",
			action, language, best.backend, best.model, best.score)
	}

	return best.backend, best.model, nil
}

func modeAllowsBackend(mode, backendName string, setup *Setup) bool {
	backend, configured := setup.Backend[backendName]
	isLocal := backendName == "ollama" || (configured && backend.Type == "ollama")
	switch mode {
	case "local":
		return isLocal
	case "cloud":
		return !isLocal
	default:
		return true
	}
}

func explicitlyConfiguredModelForAction(backend BackendConfig, profile string, action ActionType) string {
	models := backend.AgentModels
	if profile != "" && profile != "default" {
		if configuredProfile, ok := backend.Profiles[profile]; ok {
			models = configuredProfile.AgentModels
		}
	}
	switch action {
	case ActionEdit:
		return models.Editor
	case ActionResearch:
		return models.Research
	case ActionPlan, ActionReview, ActionRoute:
		return models.Query
	default:
		return ""
	}
}

func configuredModelForAction(backend BackendConfig, profile string, action ActionType) string {
	switch action {
	case ActionEdit:
		return backend.GetModelForAgentWithProfile("editor", profile)
	case ActionResearch:
		return backend.GetModelForAgentWithProfile("research", profile)
	case ActionPlan, ActionReview, ActionRoute:
		return backend.GetModelForAgentWithProfile("query", profile)
	default:
		return ""
	}
}

// trySelectApprovedModel attempts to select a specific approved model
func (ms *ModelSelector) trySelectApprovedModel(modelID string, action ActionType, language string, complexity string) (backend string, model string, err error) {
	// Parse model ID to find backend (e.g., "google/gemini-2.5-flash-lite" -> backend "openrouter")
	parts := strings.Split(modelID, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid model ID format: %s", modelID)
	}

	// Prefer configured backends. Provider-qualified model IDs such as
	// "openai/o3-mini" are also valid OpenRouter model IDs and must not be sent
	// to an unconfigured provider merely because its prefix happens to match.
	configuredBackends := make([]string, 0, len(ms.setup.Backend))
	if defaultBackend := ms.setup.Defaults.Backend; defaultBackend != "" {
		if _, ok := ms.setup.Backend[defaultBackend]; ok {
			configuredBackends = append(configuredBackends, defaultBackend)
		}
	}
	for backend := range ms.setup.Backend {
		if backend != ms.setup.Defaults.Backend {
			configuredBackends = append(configuredBackends, backend)
		}
	}

	for _, backend := range configuredBackends {
		models := ms.catalog[backend]
		for _, modelInfo := range models {
			if modelInfo.ID == modelID {
				// Check if model supports tools for edit/review actions
				if action == ActionEdit || action == ActionReview {
					if !modelInfo.Capabilities.SupportsTools {
						if os.Getenv("GPTCODE_DEBUG") == "1" {
							fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Approved model %s rejected: no tool support\n", modelID)
						}
						return "", "", fmt.Errorf("model %s does not support tools", modelID)
					}
				}
				return backend, modelID, nil
			}
		}
	}

	return "", "", fmt.Errorf("model %s not found in catalog", modelID)
}

// logBlockedNotification logs when all models fail
func (ms *ModelSelector) logBlockedNotification(action, language string) {
	if ms.setup == nil || !ms.setup.IsBlockedNotificationEnabled() {
		return
	}

	fmt.Printf("\n🔔 [BLOCKED] GT needs intervention!\n")
	fmt.Printf("   Action: %s\n", action)
	fmt.Printf("   Language: %s\n", language)
	fmt.Printf("   All approved models failed.\n")
	fmt.Printf("   Add a new approved model in setup.yaml or check API credentials.\n")
	fmt.Printf("   Check Live dashboard for more details.\n\n")

	// Save to notifications file for Live
	ms.saveBlockedNotification(action, language)
}

// saveBlockedNotification saves notification to file for Live to pick up
func (ms *ModelSelector) saveBlockedNotification(action, language string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	notifyDir := filepath.Join(homeDir, ".gptcode", "notifications")
	if err := os.MkdirAll(notifyDir, 0755); err != nil {
		return
	}

	notification := map[string]interface{}{
		"type":      "blocked",
		"action":    action,
		"language":  language,
		"timestamp": time.Now().Format(time.RFC3339),
		"message":   "All approved models failed",
	}

	data, _ := json.MarshalIndent(notification, "", "  ")
	filename := filepath.Join(notifyDir, fmt.Sprintf("blocked-%d.json", time.Now().Unix()))
	_ = os.WriteFile(filename, data, 0644)
}

func (ms *ModelSelector) scoreModel(model ModelInfo, action ActionType, language string, complexity string) float64 {
	// CRITICAL: For edit/review actions, model MUST support tools
	if action == ActionEdit || action == ActionReview {
		if !model.Capabilities.SupportsTools {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Model %s/%s rejected: no tool calling support\n", model.Backend, model.ID)
			}
			return 0
		}
		if !model.Capabilities.SupportsFileOperations {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] Model %s/%s rejected: no file operations support\n", model.Backend, model.ID)
			}
			return 0
		}
	}

	score := 100.0

	// PRIORITY 0: recommended_for from catalog - this is the pre-trained recommendation
	actionStr := strings.ToLower(string(action))
	for _, rec := range model.RecommendedFor {
		recLower := strings.ToLower(rec)
		// Map action types to catalog recommendation tags
		if (actionStr == "edit" && (recLower == "editor" || recLower == "edit")) ||
			(actionStr == "review" && (recLower == "review" || recLower == "reviewer")) ||
			(actionStr == "plan" && (recLower == "plan" || recLower == "planner" || recLower == "query")) ||
			(actionStr == "research" && (recLower == "research" || recLower == "query")) ||
			(actionStr == "route" && recLower == "router") {
			score += 100 // Strong boost for catalog-recommended models
			break
		}
	}

	// PRIORITY 1: Context window - models with larger context are significantly better
	if model.ContextWindow > 0 {
		score += (float64(model.ContextWindow) / 100000.0) * 50 // Was 10, now 50
	}

	// PRIORITY 2: Compound/Auto models that do internal routing get a boost
	modelLower := strings.ToLower(model.ID)
	if strings.Contains(modelLower, "compound") || strings.Contains(modelLower, "auto") || strings.Contains(modelLower, "router") {
		score += 75 // Compound models use best model internally
	}

	// PRIORITY 3: Speed bonus
	if model.TokensPerSec > 0 {
		score += (float64(model.TokensPerSec) / 100.0) * 5
	}

	// Rate limit penalty
	usage := ms.getTodayUsage(model.Backend, model.ID)
	if model.RateLimitDaily > 0 {
		utilization := float64(usage.Requests) / float64(model.RateLimitDaily)
		score -= utilization * 50
		if utilization >= 0.9 {
			score -= 50
		}
	}

	if usage.LastError != "" {
		score -= 30
	}

	// Cost penalty (smaller weight)
	if model.CostPer1M > 0 {
		score -= (model.CostPer1M / 10.0) * 20 // Was 30, now 20
	}

	// Feedback with CAP at ±100 total
	feedbackScore := 0.0
	for _, fb := range ms.feedback {
		if fb.ModelID != model.ID {
			continue
		}
		if fb.Action == action && strings.EqualFold(fb.Language, language) {
			if fb.Success {
				feedbackScore += 5 // Was 20, now 5 per feedback
			} else {
				feedbackScore -= 10 // Was 40, now 10 per feedback
			}
		}
	}
	// Cap feedback contribution
	if feedbackScore > 100 {
		feedbackScore = 100
	} else if feedbackScore < -100 {
		feedbackScore = -100
	}
	score += feedbackScore

	// ML recommender
	if ms.recommender != nil {
		successProb := ms.recommender.PredictSuccess(
			model.ID, action, language, complexity,
			model.ContextWindow, model.CostPer1M,
		)
		score += successProb * 50
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[MODEL_SELECTOR] ML prediction for %s: %.2f%%\n", model.ID, successProb*100)
		}
	}

	return score
}
