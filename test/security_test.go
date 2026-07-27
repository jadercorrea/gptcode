package test

import (
	"context"
	"strings"
	"testing"

	"github.com/jadercorrea/gptcode/internal/agents"
	"github.com/jadercorrea/gptcode/internal/security"
)

func TestSecurityScanner(t *testing.T) {
	provider := &agents.MockProvider{
		Response: "Test response",
	}

	scanner := security.NewScanner(provider, "test-model", ".")

	report, err := scanner.ScanAndFix(context.Background(), false)
	if err != nil {
		t.Logf("scan returned error (expected if no security tools installed): %v", err)
	}

	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !strings.Contains(string(report.Language), "go") &&
		!strings.Contains(string(report.Language), "unknown") {
		t.Errorf("unexpected language: %s", report.Language)
	}
}
