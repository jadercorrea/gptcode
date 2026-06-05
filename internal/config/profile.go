package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	Philosophy []string        `yaml:"philosophy"`
	Languages  []string        `yaml:"languages"`
	Style      map[string]any  `yaml:"style"`
	Defaults   ProfileDefaults `yaml:"defaults"`
}

type Setup struct {
	Defaults struct {
		Mode               string  `yaml:"mode,omitempty"` // local or cloud
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
	} `yaml:"defaults"`
	E2E struct {
		DefaultProfile string `yaml:"default_profile,omitempty"`
		Timeout        int    `yaml:"timeout,omitempty"`
		Notify         bool   `yaml:"notify,omitempty"`
		Parallel       int    `yaml:"parallel,omitempty"`
	} `yaml:"e2e,omitempty"`
	Backend        map[string]BackendConfig `yaml:"backend"`
	ApprovedModels []ApprovedModel          `yaml:"approved_models,omitempty"`
	Notifications  NotificationConfig       `yaml:"notifications,omitempty"`
}

type ApprovedModel struct {
	Model      string   `yaml:"model"`
	ForActions []string `yaml:"for_actions"`
	Reason     string   `yaml:"reason,omitempty"`
}

type NotificationConfig struct {
	Blocked BlockedNotificationConfig `yaml:"blocked"`
}

type BlockedNotificationConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Channels []string `yaml:"channels"` // telegram, live, both
	Message  string   `yaml:"message"`
}

type BackendConfig struct {
	Type         string                   `yaml:"type"`
	BaseURL      string                   `yaml:"base_url"`
	DefaultModel string                   `yaml:"default_model"`
	Models       map[string]string        `yaml:"models"`
	AgentModels  AgentModels              `yaml:"agent_models,omitempty"`
	Profiles     map[string]ProfileConfig `yaml:"profiles,omitempty"`
}

type ProfileConfig struct {
	AgentModels AgentModels `yaml:"agent_models"`
}

type AgentModels struct {
	Router   string `yaml:"router,omitempty"`
	Query    string `yaml:"query,omitempty"`
	Editor   string `yaml:"editor,omitempty"`
	Research string `yaml:"research,omitempty"`
}

type ProfileDefaults struct {
	Backend string `yaml:"backend"`
	Model   string `yaml:"model"`
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".gptcode")
}

func LoadProfile() (*Profile, error) {
	path := filepath.Join(configDir(), "profile.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return &Profile{}, err
	}
	var p Profile
	if err := yaml.Unmarshal(b, &p); err != nil {
		return &Profile{}, err
	}
	return &p, nil
}

func (bc *BackendConfig) GetModelForAgent(agentType string) string {
	return bc.GetModelForAgentWithProfile(agentType, "")
}

func (bc *BackendConfig) GetModelForAgentWithProfile(agentType string, profileName string) string {
	var agentModels AgentModels

	if profileName != "" && profileName != "default" {
		if profile, ok := bc.Profiles[profileName]; ok {
			agentModels = profile.AgentModels
		} else {
			agentModels = bc.AgentModels
		}
	} else {
		agentModels = bc.AgentModels
	}

	switch agentType {
	case "router":
		if agentModels.Router != "" {
			return agentModels.Router
		}
	case "query":
		if agentModels.Query != "" {
			return agentModels.Query
		}
	case "editor":
		if agentModels.Editor != "" {
			return agentModels.Editor
		}
	case "research":
		if agentModels.Research != "" {
			return agentModels.Research
		}
	}
	return bc.DefaultModel
}

// ResolveBackendAndModel determines the backend for a model
// Model strings can be:
// - "llama-3.3-70b" -> uses defaultBackend
// - "groq/compound" -> model slug for the current backend (groq), use as-is
// - "moonshotai/kimi-k2" -> model slug for the current backend, use as-is
// We only change backend if the prefix is a DIFFERENT backend than default
func (s *Setup) ResolveBackendAndModel(modelStr string, defaultBackend string) (backend string, model string) {
	// Always use the defaultBackend and full model string
	// The model string itself is the slug that the API expects
	return defaultBackend, modelStr
}

// GetApprovedModelsForAction returns approved models for a specific action
func (s *Setup) GetApprovedModelsForAction(action string) []ApprovedModel {
	var result []ApprovedModel
	for _, model := range s.ApprovedModels {
		for _, a := range model.ForActions {
			if a == action {
				result = append(result, model)
				break
			}
		}
	}
	return result
}

// IsBlockedNotificationEnabled checks if blocked notifications are enabled
func (s *Setup) IsBlockedNotificationEnabled() bool {
	return s.Notifications.Blocked.Enabled
}
