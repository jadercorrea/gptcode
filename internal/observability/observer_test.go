package observability

import (
	"testing"
	"time"
)

func TestSummaryTracksToolDurationSeparatelyFromTaskDuration(t *testing.T) {
	observer := NewObserver()
	observer.Emit(&ToolCallEvent{Name: "apply_patch", Duration: 25 * time.Millisecond})

	summary := observer.Summary()
	if summary.ToolDuration != 25*time.Millisecond {
		t.Fatalf("expected 25ms of tool time, got %s", summary.ToolDuration)
	}
}

func TestFinalOutcomeOverridesRecoverableIntermediateErrors(t *testing.T) {
	observer := NewObserver()
	observer.Emit(&ToolCallEvent{Name: "run_command", Error: "tests failed"})
	observer.SetOutcome(true)

	summary := observer.Summary()
	if !summary.Success {
		t.Fatal("successful final outcome must not be reported as failed")
	}
	if len(summary.Errors) != 1 {
		t.Fatalf("errors = %v, want recoverable error retained", summary.Errors)
	}
}

func TestSummaryWithErrorFailsBeforeFinalOutcome(t *testing.T) {
	observer := NewObserver()
	observer.Emit(&ToolCallEvent{Name: "run_command", Error: "tests failed"})

	if observer.Summary().Success {
		t.Fatal("intermediate errors must fail a summary without a final outcome")
	}
}
