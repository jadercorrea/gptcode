package modes

import "github.com/jadercorrea/gptcode/internal/config"

func configuredAgentModel(backend config.BackendConfig, profile, agent string) string {
	return backend.GetModelForAgentWithProfile(agent, profile)
}
