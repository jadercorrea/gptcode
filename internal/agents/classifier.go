package agents

import (
	"context"
	"fmt"

	"github.com/jadercorrea/gptcode/internal/ml"
)

type Intent string

const (
	IntentQuery    Intent = "query"
	IntentEdit     Intent = "edit"
	IntentResearch Intent = "research"
	IntentTest     Intent = "test"
	IntentReview   Intent = "review"
)

type Classifier struct{}

func NewClassifier(_ interface{}, _ string) *Classifier {
	return &Classifier{}
}

func (c *Classifier) ClassifyIntent(ctx context.Context, userMessage string) (Intent, error) {
	p, err := ml.LoadEmbedded("intent")
	if err != nil {
		return IntentEdit, fmt.Errorf("failed to load intent model: %w", err)
	}

	label, _ := p.Predict(userMessage)

	intent := mapMLLabelToIntent(label)
	if intent == "" {
		return IntentEdit, fmt.Errorf("unknown ML label: %s", label)
	}

	return intent, nil
}

func mapMLLabelToIntent(label string) Intent {
	switch label {
	case "router":
		return IntentQuery
	case "query":
		return IntentQuery
	case "editor":
		return IntentEdit
	case "research":
		return IntentResearch
	case "review":
		return IntentReview
	default:
		return ""
	}
}
