package modes

import (
	"testing"

	"github.com/jadercorrea/gptcode/internal/config"
)

func TestConfiguredAgentModelUsesActiveBackendProfile(t *testing.T) {
	backend := config.BackendConfig{
		DefaultModel: "fallback",
		Profiles: map[string]config.ProfileConfig{
			"evaluation": {
				AgentModels: config.AgentModels{
					Query:    "qwen-query",
					Research: "qwen-research",
				},
			},
		},
	}

	if got := configuredAgentModel(backend, "evaluation", "query"); got != "qwen-query" {
		t.Fatalf("query model = %q, want profile model", got)
	}
	if got := configuredAgentModel(backend, "evaluation", "research"); got != "qwen-research" {
		t.Fatalf("research model = %q, want profile model", got)
	}
}
