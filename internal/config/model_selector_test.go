package config

import (
	"testing"
)

func TestModelSelectorScoring(t *testing.T) {
	setup := &Setup{
		Defaults: struct {
			Mode               string  `yaml:"mode,omitempty"`
			Backend            string  `yaml:"backend"`
			Profile            string  `yaml:"profile,omitempty"`
			Model              string  `yaml:"model,omitempty"`
			Lang               string  `yaml:"lang"`
			SystemPromptFile   string  `yaml:"system_prompt_file,omitempty"`
			MLComplexThreshold float64 `yaml:"ml_complex_threshold,omitempty"`
			MLIntentThreshold  float64 `yaml:"ml_intent_threshold,omitempty"`
			GraphMaxFiles      int     `yaml:"graph_max_files,omitempty"`
			BudgetMode         bool    `yaml:"budget_mode,omitempty"`
			MaxCostPerTask     float64 `yaml:"max_cost_per_task,omitempty"`
			MonthlyBudget      float64 `yaml:"monthly_budget,omitempty"`
			CavemanMode        string  `yaml:"caveman_mode,omitempty"`
		}{
			Mode:    "cloud",
			Backend: "openrouter",
		},
	}

	selector := &ModelSelector{
		catalog: map[string][]ModelInfo{
			"openrouter": {
				{
					ID:             "google/gemini-2.0-flash-exp:free",
					CostPer1M:      0,
					RateLimitDaily: 1000,
					ContextWindow:  1000000,
					TokensPerSec:   150,
					Backend:        "openrouter",
					Capabilities: ModelCapabilities{
						SupportsTools:          true,
						SupportsFileOperations: true,
					},
				},
				{
					ID:             "qwen/qwen-2.5-coder-32b-instruct",
					CostPer1M:      0.3,
					RateLimitDaily: 10000,
					ContextWindow:  32768,
					TokensPerSec:   120,
					Backend:        "openrouter",
					Capabilities: ModelCapabilities{
						SupportsTools:          true,
						SupportsFileOperations: true,
					},
				},
			},
			"groq": {
				{
					ID:             "llama-3.3-70b-versatile",
					CostPer1M:      0.59,
					RateLimitDaily: 14400,
					ContextWindow:  131072,
					TokensPerSec:   300,
					Backend:        "groq",
					Capabilities: ModelCapabilities{
						SupportsTools:          true,
						SupportsFileOperations: true,
					},
				},
			},
		},
		feedback: []ModelFeedback{},
		usage:    make(map[string]map[string]ModelUsage),
		setup:    setup,
	}

	backend, model, err := selector.SelectModel(ActionEdit, "go", "simple")
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}

	if backend != "openrouter" {
		t.Errorf("Expected backend=openrouter, got %s", backend)
	}

	if model != "google/gemini-2.0-flash-exp:free" {
		t.Errorf("Expected free Gemini model, got %s", model)
	}

	t.Logf("Selected: %s/%s", backend, model)
}
