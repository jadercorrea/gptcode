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
