package catalog

import (
	"encoding/json"
	"fmt"
	"github.com/jadercorrea/gptcode/internal/feedback"
	"github.com/jadercorrea/gptcode/internal/ollama"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GetCatalogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gptcode", "models_catalog.json")
}

func Load() (*OutputJSON, error) {
	path := GetCatalogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		// Fallback to embedded default catalog
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "[CATALOG] User catalog not found, using embedded default_models.json")
		}
		data = defaultModelsJSON
		if len(data) == 0 {
			return nil, fmt.Errorf("no catalog available: user catalog not found and embedded catalog is empty")
		}
	}

	var catalog OutputJSON
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse catalog: %w", err)
	}

	return &catalog, nil
}

func GetModelsForBackend(backend string) ([]ModelOutput, error) {
	catalog, err := Load()
	if err != nil {
		return nil, err
	}

	var models []ModelOutput
	switch strings.ToLower(backend) {
	case "groq":
		models = catalog.Groq.Models
	case "openrouter":
		models = catalog.OpenRouter.Models
	case "ollama":
		models = catalog.Ollama.Models
	case "openai":
		models = catalog.OpenAI.Models
	case "deepseek":
		models = catalog.DeepSeek.Models
	default:
		return nil, fmt.Errorf("unknown backend: %s", backend)
	}

	return models, nil
}

func FilterByTag(models []ModelOutput, tag string) []ModelOutput {
	filtered := []ModelOutput{}
	tag = strings.ToLower(tag)

	for _, model := range models {
		for _, t := range model.Tags {
			if strings.ToLower(t) == tag {
				filtered = append(filtered, model)
				break
			}
		}
	}

	return filtered
}

func GetRecommendedForAgent(backend string, agent string) ([]ModelOutput, error) {
	models, err := GetModelsForBackend(backend)
	if err != nil {
		return nil, err
	}

	filtered := []ModelOutput{}
	for _, model := range models {
		for _, rec := range model.RecommendedFor {
			if rec == agent {
				filtered = append(filtered, model)
				break
			}
		}
	}

	return filtered, nil
}

func getFeedbackScores(agent string) map[string]float64 {
	events, err := feedback.LoadAll()
	if err != nil || len(events) == 0 {
		return map[string]float64{}
	}

	stats := feedback.Analyze(events)
	scores := make(map[string]float64)

	for model, modelStats := range stats.ByModel {
		if modelStats.Total >= 3 {
			scores[model] = modelStats.Ratio
		}
	}

	return scores
}

func SearchModelsMulti(backend string, queryTerms []string, agent string) ([]ModelOutput, error) {
	catalog, err := Load()
	if err != nil {
		return nil, err
	}

	backendLower := strings.ToLower(backend)

	if backendLower == "" && len(queryTerms) > 0 {
		firstTerm := strings.ToLower(queryTerms[0])
		if firstTerm == "groq" || firstTerm == "openrouter" || firstTerm == "ollama" || firstTerm == "openai" || firstTerm == "deepseek" {
			backendLower = firstTerm
			queryTerms = queryTerms[1:]
		}
	}

	var allModels []ModelOutput

	switch backendLower {
	case "groq":
		allModels = catalog.Groq.Models
	case "openrouter":
		allModels = catalog.OpenRouter.Models
	case "ollama":
		allModels = catalog.Ollama.Models
	case "openai":
		allModels = catalog.OpenAI.Models
	case "deepseek":
		allModels = catalog.DeepSeek.Models
	case "":
		allModels = append(allModels, catalog.Groq.Models...)
		allModels = append(allModels, catalog.OpenRouter.Models...)
		allModels = append(allModels, catalog.Ollama.Models...)
		allModels = append(allModels, catalog.OpenAI.Models...)
		allModels = append(allModels, catalog.DeepSeek.Models...)
	default:
		return nil, fmt.Errorf("unknown backend: %s", backend)
	}

	var filtered []ModelOutput

	for _, model := range allModels {
		matches := true

		for _, term := range queryTerms {
			termLower := strings.ToLower(term)
			nameLower := strings.ToLower(model.Name)
			idLower := strings.ToLower(model.ID)

			termMatches := strings.Contains(nameLower, termLower) ||
				strings.Contains(idLower, termLower)

			for _, tag := range model.Tags {
				if strings.ToLower(tag) == termLower {
					termMatches = true
					break
				}
			}

			if !termMatches {
				matches = false
				break
			}
		}

		if matches {
			filtered = append(filtered, model)
		}
	}

	if len(queryTerms) == 0 && agent != "" {
		var agentFiltered []ModelOutput
		for _, model := range filtered {
			for _, rec := range model.RecommendedFor {
				if rec == agent {
					agentFiltered = append(agentFiltered, model)
					break
				}
			}
		}

		if len(agentFiltered) > 0 {
			filtered = agentFiltered
		}
	}

	for i := range filtered {
		if backendLower == "ollama" || strings.HasPrefix(strings.ToLower(filtered[i].ID), "ollama/") {
			modelName := strings.TrimPrefix(filtered[i].ID, "ollama/")
			installed, err := ollama.IsInstalled(modelName)
			if err == nil {
				filtered[i].Installed = installed
			}
		}
	}

	feedbackScores := getFeedbackScores(agent)
	for i := range filtered {
		if score, ok := feedbackScores[filtered[i].ID]; ok {
			filtered[i].FeedbackScore = score
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].FeedbackScore != filtered[j].FeedbackScore {
			return filtered[i].FeedbackScore > filtered[j].FeedbackScore
		}
		costA := filtered[i].PricingPrompt + filtered[i].PricingComp
		costB := filtered[j].PricingPrompt + filtered[j].PricingComp

		if costA != costB {
			return costA < costB
		}

		if filtered[i].ContextWindow != filtered[j].ContextWindow {
			return filtered[i].ContextWindow > filtered[j].ContextWindow
		}

		return filtered[i].Name < filtered[j].Name
	})

	return filtered, nil
}

func SearchModels(backend string, query string, agent string) ([]ModelOutput, error) {
	models, err := GetModelsForBackend(backend)
	if err != nil {
		return nil, err
	}

	var filtered []ModelOutput

	if query != "" {
		queryLower := strings.ToLower(query)
		for _, model := range models {
			nameLower := strings.ToLower(model.Name)
			idLower := strings.ToLower(model.ID)

			if strings.Contains(nameLower, queryLower) || strings.Contains(idLower, queryLower) {
				filtered = append(filtered, model)
			}
		}
	} else if agent != "" {
		for _, model := range models {
			for _, rec := range model.RecommendedFor {
				if rec == agent {
					filtered = append(filtered, model)
					break
				}
			}
		}

		if len(filtered) == 0 {
			filtered = models
		}
	} else {
		filtered = models
	}

	sort.Slice(filtered, func(i, j int) bool {
		costA := filtered[i].PricingPrompt + filtered[i].PricingComp
		costB := filtered[j].PricingPrompt + filtered[j].PricingComp

		if costA != costB {
			return costA < costB
		}

		if filtered[i].ContextWindow != filtered[j].ContextWindow {
			return filtered[i].ContextWindow > filtered[j].ContextWindow
		}

		return filtered[i].Name < filtered[j].Name
	})

	return filtered, nil
}
