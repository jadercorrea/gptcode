package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// LoopDetector implements Claude Code-style intelligent loop detection.
// It detects:
// 1. Tool Call Loops: same tool+args called N times consecutively
// 2. Content Loops ("Chanting"): same response repeated N times
// 3. Intent-aware iteration limits
type LoopDetector struct {
	// Configuration thresholds (inspired by Claude Code)
	ToolLoopThreshold    int // Default: 5 consecutive identical tool calls
	ContentLoopThreshold int // Default: 3 consecutive identical responses

	// State tracking
	toolCallHistory    []string // Hashes of tool calls
	responseHistory    []string // Hashes of responses
	consecutiveToolHit int      // Count of same tool in a row
	lastToolHash       string

	// Progress tracking
	Iteration         int
	FileModifications int
	ReadOperations    int
	Intent            string // "query", "edit", "plan", "research"
	MaxIterations     int    // Optional caller-provided limit
}

// NewLoopDetector creates a new loop detector with Claude Code-style thresholds
func NewLoopDetector(intent string) *LoopDetector {
	return &LoopDetector{
		ToolLoopThreshold:    3, // Claude Code uses 5, but we reduce to 3 to save costs
		ContentLoopThreshold: 2, // More aggressive than Claude's 10
		toolCallHistory:      make([]string, 0),
		responseHistory:      make([]string, 0),
		Intent:               intent,
	}
}

// RecordToolCall records a tool call and checks for loops
func (ld *LoopDetector) RecordToolCall(toolName, arguments string) (isLoop bool, reason string) {
	hash := ld.hash(fmt.Sprintf("%s:%s", toolName, arguments))
	ld.toolCallHistory = append(ld.toolCallHistory, hash)

	if hash == ld.lastToolHash {
		ld.consecutiveToolHit++
	} else {
		ld.consecutiveToolHit = 1
		ld.lastToolHash = hash
	}

	if ld.consecutiveToolHit >= ld.ToolLoopThreshold {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[LOOP_DETECTOR] Tool loop detected: %s called %d times consecutively\n", toolName, ld.consecutiveToolHit)
		}
		return true, fmt.Sprintf("Tool loop: %s called %d times with same arguments", toolName, ld.consecutiveToolHit)
	}

	return false, ""
}

// RecordResponse records a model response and checks for content loops
func (ld *LoopDetector) RecordResponse(response string) (isLoop bool, reason string) {
	hash := ld.hash(response)
	ld.responseHistory = append(ld.responseHistory, hash)

	// Check for consecutive identical responses
	consecutiveCount := 1
	for i := len(ld.responseHistory) - 2; i >= 0 && i >= len(ld.responseHistory)-ld.ContentLoopThreshold; i-- {
		if ld.responseHistory[i] == hash {
			consecutiveCount++
		} else {
			break
		}
	}

	if consecutiveCount >= ld.ContentLoopThreshold {
		if os.Getenv("GPTCODE_DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[LOOP_DETECTOR] Content loop detected: same response %d times\n", consecutiveCount)
		}
		return true, fmt.Sprintf("Content loop: identical response repeated %d times", consecutiveCount)
	}

	return false, ""
}

// ShouldContinue checks if the loop should continue based on all conditions
func (ld *LoopDetector) ShouldContinue() (shouldContinue bool, reason string) {
	ld.Iteration++

	// Intent-aware safety limits (inspired by Claude Code --max-turns)
	maxIterations := ld.getMaxIterationsForIntent()

	if ld.Iteration > maxIterations {
		return false, fmt.Sprintf("Safety limit reached: %d iterations for %s intent", maxIterations, ld.Intent)
	}

	// Intent-aware progress checks
	switch ld.Intent {
	case "edit":
		// For edit tasks: abort if no file modifications after 7 iterations
		if ld.Iteration >= 7 && ld.FileModifications == 0 {
			if os.Getenv("GPTCODE_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "[LOOP_DETECTOR] Fatal: edit task with no file modifications after %d iterations\n", ld.Iteration)
			}
			return false, fmt.Sprintf("Safety abort: %d iterations without file modifications", ld.Iteration)
		}

	case "query", "research":
		// For query/research: progress is measured by reads, not writes
		// No early stop based on file modifications
	}

	return true, ""
}

// getMaxIterationsForIntent returns the maximum iterations based on task intent
func (ld *LoopDetector) getMaxIterationsForIntent() int {
	if ld.MaxIterations > 0 {
		return ld.MaxIterations
	}
	limits := map[string]int{
		"query":    10, // Query tasks are typically shorter
		"edit":     15, // Reduced from 25 to prevent runaway edits
		"plan":     10, // Planning is moderate
		"research": 15, // Research can be extensive but capped
		"":         15, // Default fallback
	}

	if limit, ok := limits[ld.Intent]; ok {
		return limit
	}
	return limits[""]
}

func (ld *LoopDetector) SetMaxIterations(max int) {
	ld.MaxIterations = max
}

// RecordFileModification increments the file modification counter
func (ld *LoopDetector) RecordFileModification() {
	ld.FileModifications++
}

// RecordReadOperation increments the read operation counter
func (ld *LoopDetector) RecordReadOperation() {
	ld.ReadOperations++
}

// GetStats returns current loop detector statistics
func (ld *LoopDetector) GetStats() string {
	return fmt.Sprintf("Iterations: %d, Files Modified: %d, Files Read: %d, Tool Calls: %d",
		ld.Iteration, ld.FileModifications, ld.ReadOperations, len(ld.toolCallHistory))
}

// hash creates a short hash of the input for comparison
func (ld *LoopDetector) hash(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:8]) // Use first 8 bytes for shorter comparison
}
