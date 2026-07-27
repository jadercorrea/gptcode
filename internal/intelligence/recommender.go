package intelligence

import (
	"fmt"
	"github.com/jadercorrea/gptcode/internal/config"
	"os"
)

type ModelRecommendation struct {
	Backend    string
	Model      string
	Reason     string
	Confidence float64
	Score      float64
	Metrics    RecommendationMetrics
}

type RecommendationMetrics struct {
	SuccessRate  float64
	AvgLatencyMs int64
	CostPer1M    float64
	Availability float64
	SpeedTPS     int
}

var DefaultCatalog = NewModelCatalog()

func RecommendModelForRetry(setup *config.Setup, agentType string, failedBackend string, failedModel string, task string) ([]ModelRecommendation, error) {
	recommendations := make([]ModelRecommendation, 0)

	history, err := GetRecentModelPerformance("", 100)
	if err != nil {
		history = []ModelSuccess{}
	}

	historyMap := make(map[string]ModelSuccess)
	latencyMap := make(map[string]int64)
	for _, h := range history {
		key := h.Backend + "/" + h.Model
		historyMap[key] = h
		if h.AvgLatency > 0 {
			latencyMap[key] = h.AvgLatency
		}
	}

	// Use catalog instead of hardcoded map
	candidateModels := DefaultCatalog.GetModelsForAgent(agentType)

	mode := setup.Defaults.Mode

	for _, modelInfo := range candidateModels {
		backend := modelInfo.Backend
		model := modelInfo.Name

		if backend == failedBackend && model == failedModel {
			continue
		}

		backendCfg, backendExists := setup.Backend[backend]
		if !backendExists {
			continue
		}

		// Filter by mode
		if mode == "local" && backend != "ollama" {
			continue
		}
		if mode == "cloud" && backend == "ollama" {
			continue
		}

		// Budget constraint check
		if setup.Defaults.BudgetMode {
			if modelInfo.CostPer1M > setup.Defaults.MaxCostPerTask && setup.Defaults.MaxCostPerTask > 0 {
				continue // Skip models that exceed per-task cost limit
			}
		}

		key := backend + "/" + model
		h, hasHistory := historyMap[key]

		metrics := RecommendationMetrics{
			SuccessRate:  0.5,
			Availability: 1.0,
			CostPer1M:    modelInfo.CostPer1M,
			SpeedTPS:     modelInfo.SpeedTPS,
		}

		if hasHistory && h.TotalTasks >= 3 {
			metrics.SuccessRate = h.SuccessRate
		}

		if latency, ok := latencyMap[key]; ok {
			metrics.AvgLatencyMs = latency
		}

		score := calculateScore(metrics, backend == failedBackend)

		// Apply additional budget penalty if in budget mode
		if setup.Defaults.BudgetMode {
			// Boost score for free/local models in budget mode
			if modelInfo.CostPer1M == 0.0 {
				score *= 1.2 // 20% boost for free models
			}
			if modelInfo.Backend == "ollama" {
				score *= 1.1 // 10% boost for local models
			}
		}

		confidence := metrics.SuccessRate

		reason := buildReason(metrics, hasHistory, h.TotalTasks)

		modelCfg, modelExists := backendCfg.Models[model]
		if !modelExists {
			modelCfg = model
		}

		recommendations = append(recommendations, ModelRecommendation{
			Backend:    backend,
			Model:      modelCfg,
			Reason:     reason,
			Confidence: confidence,
			Score:      score,
			Metrics:    metrics,
		})
	}

	sortByScore(recommendations)

	return recommendations, nil
}

func calculateScore(m RecommendationMetrics, sameBackend bool) float64 {
	successWeight := 0.5
	speedWeight := 0.2
	costWeight := 0.2
	availWeight := 0.1

	successScore := m.SuccessRate

	speedScore := 0.5
	if m.SpeedTPS > 0 {
		speedScore = min(float64(m.SpeedTPS)/1000.0, 1.0)
	}
	if m.AvgLatencyMs > 0 {
		latencyScore := 1.0 - min(float64(m.AvgLatencyMs)/10000.0, 0.5)
		speedScore = (speedScore + latencyScore) / 2
	}

	costScore := 1.0
	if m.CostPer1M > 0 {
		costScore = 1.0 - min(m.CostPer1M/2.0, 0.8)
	}

	availScore := m.Availability

	score := successWeight*successScore +
		speedWeight*speedScore +
		costWeight*costScore +
		availWeight*availScore

	if sameBackend {
		score *= 0.95
	}

	return score
}

func buildReason(m RecommendationMetrics, hasHistory bool, totalTasks int) string {
	if hasHistory && totalTasks >= 3 {
		return fmt.Sprintf("Success: %.0f%% (%d tasks), Speed: %d TPS, Cost: $%.2f/1M",
			m.SuccessRate*100, totalTasks, m.SpeedTPS, m.CostPer1M)
	}
	return fmt.Sprintf("Known capable, Speed: %d TPS, Cost: $%.2f/1M",
		m.SpeedTPS, m.CostPer1M)
}

func sortByScore(recs []ModelRecommendation) {
	for i := 0; i < len(recs)-1; i++ {
		for j := i + 1; j < len(recs); j++ {
			if recs[j].Score > recs[i].Score {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func PromptUserForModelSelection(recommendations []ModelRecommendation, agentType string) (ModelRecommendation, error) {
	if len(recommendations) == 0 {
		return ModelRecommendation{}, fmt.Errorf("no recommendations available")
	}

	if len(recommendations) == 1 {
		return recommendations[0], nil
	}

	if len(recommendations) >= 2 {
		diff := recommendations[0].Score - recommendations[1].Score
		if diff < 0.1 {
			fmt.Fprintf(os.Stderr, "\n🤔 Multiple good options for %s agent:\n\n", agentType)
			for i, rec := range recommendations[:minInt(3, len(recommendations))] {
				fmt.Fprintf(os.Stderr, "  [%d] %s/%s\n", i+1, rec.Backend, rec.Model)
				fmt.Fprintf(os.Stderr, "      Score: %.2f | %s\n", rec.Score, rec.Reason)
			}

			fmt.Fprintf(os.Stderr, "\nSelect option (1-%d) or press Enter for auto-select: ", minInt(3, len(recommendations)))

			var input string
			fmt.Scanln(&input)

			if input == "" {
				return recommendations[0], nil
			}

			var choice int
			fmt.Sscanf(input, "%d", &choice)
			if choice >= 1 && choice <= len(recommendations) {
				return recommendations[choice-1], nil
			}
		}
	}

	return recommendations[0], nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func SelectBestModelForAgent(setup *config.Setup, agentType string) (backend string, model string, reason string, err error) {
	// Check cache first
	mode := setup.Defaults.Mode
	if cachedBackend, cachedModel, cachedReason, found := GetCachedRecommendation(agentType, "", mode); found {
		return cachedBackend, cachedModel, cachedReason + " (cached)", nil
	}

	currentBackend := setup.Defaults.Backend
	backendCfg, ok := setup.Backend[currentBackend]
	if !ok {
		return "", "", "", fmt.Errorf("backend %s not configured", currentBackend)
	}

	configuredModel := backendCfg.GetModelForAgent(agentType)
	if configuredModel != "" && configuredModel != backendCfg.DefaultModel {
		return currentBackend, configuredModel, "User configured", nil
	}

	history, _ := GetRecentModelPerformance("", 100)
	historyMap := make(map[string]ModelSuccess)
	latencyMap := make(map[string]int64)
	for _, h := range history {
		key := h.Backend + "/" + h.Model
		historyMap[key] = h
		if h.AvgLatency > 0 {
			latencyMap[key] = h.AvgLatency
		}
	}

	candidates := DefaultCatalog.GetModelsForAgent(agentType)

	var bestRec ModelRecommendation
	var allRecs []ModelRecommendation

	for _, modelInfo := range candidates {
		if _, exists := setup.Backend[modelInfo.Backend]; !exists {
			continue
		}

		if mode == "local" && modelInfo.Backend != "ollama" {
			continue
		}

		if mode == "cloud" && modelInfo.Backend == "ollama" {
			continue
		}

		// Budget constraint check
		if setup.Defaults.BudgetMode {
			if modelInfo.CostPer1M > setup.Defaults.MaxCostPerTask && setup.Defaults.MaxCostPerTask > 0 {
				continue // Skip models that exceed per-task cost limit
			}
		}

		key := modelInfo.Backend + "/" + modelInfo.Name
		h, hasHistory := historyMap[key]

		metrics := RecommendationMetrics{
			SuccessRate:  0.5,
			Availability: 1.0,
			CostPer1M:    modelInfo.CostPer1M,
			SpeedTPS:     modelInfo.SpeedTPS,
		}

		if hasHistory && h.TotalTasks >= 3 {
			metrics.SuccessRate = h.SuccessRate
		}

		if latency, ok := latencyMap[key]; ok {
			metrics.AvgLatencyMs = latency
		}

		score := calculateScore(metrics, false)

		// Apply additional budget penalty if in budget mode
		if setup.Defaults.BudgetMode {
			// Boost score for free/local models in budget mode
			if modelInfo.CostPer1M == 0.0 {
				score *= 1.2 // 20% boost for free models
			}
			if modelInfo.Backend == "ollama" {
				score *= 1.1 // 10% boost for local models
			}
		}

		rec := ModelRecommendation{
			Backend:    modelInfo.Backend,
			Model:      modelInfo.Name,
			Score:      score,
			Confidence: metrics.SuccessRate,
			Metrics:    metrics,
			Reason:     buildReason(metrics, hasHistory, h.TotalTasks),
		}

		_ = append(allRecs, rec)

		if score > bestRec.Score {
			bestRec = rec
		}
	}

	if bestRec.Backend == "" {
		return currentBackend, backendCfg.DefaultModel, "Fallback to default (no suitable models)", nil
	}

	modelCfg := bestRec.Model
	if backendCfg, ok := setup.Backend[bestRec.Backend]; ok {
		if alias, exists := backendCfg.Models[bestRec.Model]; exists {
			modelCfg = alias
		}
	}

	// Cache the result
	SetCachedRecommendation(agentType, "", mode, bestRec.Backend, modelCfg, bestRec.Reason)

	return bestRec.Backend, modelCfg, bestRec.Reason, nil
}
