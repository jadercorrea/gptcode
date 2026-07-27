package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jadercorrea/gptcode/internal/live"
)

// TracerImpl is a concrete implementation of the Tracer interface
type TracerImpl struct {
	sessionID string
	command   string
	startTime time.Time
	steps     []StepTrace
	path      []string
	totalCost float64
	mutex     sync.Mutex
}

// NewTracer creates a new tracer instance
func NewTracer() Tracer {
	return &TracerImpl{
		steps: make([]StepTrace, 0),
		path:  make([]string, 0),
	}
}

// Begin starts a new session trace
func (t *TracerImpl) Begin(sessionID, command string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.sessionID = sessionID
	t.command = command
	t.startTime = time.Now()

	return nil
}

// RecordStep logs a step execution
func (t *TracerImpl) RecordStep(step StepTrace) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Add to steps
	t.steps = append(t.steps, step)

	// Add to path if not already there
	found := false
	for _, node := range t.path {
		if node == step.Node {
			found = true
			break
		}
	}
	if !found {
		t.path = append(t.path, step.Node)
	}

	// Update total cost
	t.totalCost += step.Metrics.Cost

	// Stream to Live dashboard if available
	if err := t.streamToLive(step); err != nil {
		// Log error but don't fail the operation
		fmt.Fprintf(os.Stderr, "Warning: failed to stream to Live dashboard: %v\n", err)
	}

	return nil
}

// RecordDecision logs a routing or choice decision
func (t *TracerImpl) RecordDecision(node string, decision Decision) error {
	step := StepTrace{
		Node:      node,
		Timestamp: time.Now(),
		Decision:  &decision,
	}

	return t.RecordStep(step)
}

// RecordMetrics logs performance metrics
func (t *TracerImpl) RecordMetrics(node string, metrics Metrics) error {
	step := StepTrace{
		Node:      node,
		Timestamp: time.Now(),
		Metrics:   metrics,
	}

	return t.RecordStep(step)
}

// End finalizes the session trace
func (t *TracerImpl) End(success bool) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	endTime := time.Now()
	totalTimeMs := endTime.Sub(t.startTime).Milliseconds()

	sessionTrace := SessionTrace{
		SessionID:   t.sessionID,
		Command:     t.command,
		StartTime:   t.startTime,
		EndTime:     &endTime,
		Steps:       t.steps,
		Path:        t.path,
		TotalCost:   t.totalCost,
		TotalTimeMs: totalTimeMs,
		Success:     success,
	}

	// Write to file for now (can be extended to other storage)
	if err := t.writeToFile(sessionTrace); err != nil {
		return fmt.Errorf("failed to write trace to file: %w", err)
	}

	// Stream to Live dashboard if available
	if err := t.streamToLiveSession(sessionTrace); err != nil {
		// Log error but don't fail the operation
		fmt.Fprintf(os.Stderr, "Warning: failed to stream session to Live dashboard: %v\n", err)
	}

	return nil
}

// Flush writes accumulated traces to storage
func (t *TracerImpl) Flush() error {
	// In this implementation, we write to file immediately in RecordStep and End
	// so Flush is a no-op, but could be extended to flush other storage backends
	return nil
}

// writeToFile writes the session trace to a file
func (t *TracerImpl) writeToFile(sessionTrace SessionTrace) error {
	// Create a filename with timestamp and session ID
	filename := fmt.Sprintf("trace_%s_%s.json", t.sessionID, t.startTime.Format("20060102_150405"))

	// Marshal to JSON
	data, err := json.MarshalIndent(sessionTrace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session trace: %w", err)
	}

	// Write to file
	return os.WriteFile(filename, data, 0644)
}

// streamToLive sends the step to the Live dashboard via WebSocket
func (t *TracerImpl) streamToLive(step StepTrace) error {
	// Check if Live client is available
	client := live.GetClient()
	if client == nil {
		return nil // No Live client, skip streaming
	}

	// Send human-readable execution step for dashboard cards
	stepType := "step"
	description := step.Node
	if step.Decision != nil {
		description = fmt.Sprintf("%s → %s", step.Node, step.Decision.Reasoning)
		stepType = "step"
	}
	if step.Metrics.Cost > 0 {
		description = fmt.Sprintf("%s (cost: $%.4f)", description, step.Metrics.Cost)
	}

	_ = client.SendExecutionStep(stepType, description, map[string]interface{}{
		"node":       step.Node,
		"session_id": t.sessionID,
	})

	// Also send raw trace data
	traceData := map[string]interface{}{
		"type":       "step_trace",
		"session_id": t.sessionID,
		"step":       step,
	}

	return client.SendTraceData(traceData)
}

// streamToLiveSession sends the complete session trace to the Live dashboard
func (t *TracerImpl) streamToLiveSession(sessionTrace SessionTrace) error {
	// Check if Live client is available
	client := live.GetClient()
	if client == nil {
		return nil // No Live client, skip streaming
	}

	// Prepare the session trace data to send
	traceData := map[string]interface{}{
		"type":           "session_trace",
		"session_trace":  sessionTrace,
		"command":        t.command,
		"success":        sessionTrace.Success,
		"total_duration": sessionTrace.TotalTimeMs,
		"total_cost":     sessionTrace.TotalCost,
	}

	// Send via WebSocket
	return client.SendTraceData(traceData)
}
