package observability

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gptcode/internal/intelligence"
)

// Event is the base interface for all observer events
type Event interface {
	EventType() string
	Timestamp() time.Time
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	Time time.Time `json:"timestamp"`
}

func (e BaseEvent) Timestamp() time.Time { return e.Time }

// ToolCallEvent is emitted when a tool is called
type ToolCallEvent struct {
	BaseEvent
	Name        string        `json:"name"`
	Arguments   string        `json:"arguments"`
	Result      string        `json:"result"`
	Duration    time.Duration `json:"duration_ms"`
	Error       string        `json:"error,omitempty"`
	TokensSaved int           `json:"tokens_saved,omitempty"`
	CostSaved   float64       `json:"cost_saved,omitempty"`
}

func (e ToolCallEvent) EventType() string { return "tool_call" }

// FileModifiedEvent is emitted when a file is created, modified, or deleted
type FileModifiedEvent struct {
	BaseEvent
	Path      string `json:"path"`
	Operation string `json:"operation"` // "create", "modify", "delete"
	Bytes     int64  `json:"bytes"`
}

func (e FileModifiedEvent) EventType() string { return "file_modified" }

// LLMRequestEvent is emitted for each LLM API call
type LLMRequestEvent struct {
	BaseEvent
	Model     string        `json:"model"`
	Backend   string        `json:"backend"`
	TokensIn  int           `json:"tokens_in"`
	TokensOut int           `json:"tokens_out"`
	Cost      float64       `json:"cost"`
	Duration  time.Duration `json:"duration_ms"`
	Error     string        `json:"error,omitempty"`
}

func (e LLMRequestEvent) EventType() string { return "llm_request" }

// AgentEvent is emitted when an agent starts or ends
type AgentEvent struct {
	BaseEvent
	Name    string `json:"name"`
	Phase   string `json:"phase"` // "start", "end"
	Success bool   `json:"success,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e AgentEvent) EventType() string { return "agent" }

// MovementEvent is emitted for Symphony movements
type MovementEvent struct {
	BaseEvent
	ID      string `json:"id"`
	Name    string `json:"name"`
	Phase   string `json:"phase"` // "start", "end"
	Success bool   `json:"success,omitempty"`
}

func (e MovementEvent) EventType() string { return "movement" }

// ValidationEvent is emitted after validation
type ValidationEvent struct {
	BaseEvent
	Success bool     `json:"success"`
	Issues  []string `json:"issues,omitempty"`
}

func (e ValidationEvent) EventType() string { return "validation" }

// ExecutionSummary provides a summary of the entire execution
type ExecutionSummary struct {
	Duration      time.Duration  `json:"duration_ms"`
	FilesCreated  []string       `json:"files_created"`
	FilesModified []string       `json:"files_modified"`
	FilesDeleted  []string       `json:"files_deleted"`
	ToolCalls     map[string]int `json:"tool_calls"`
	LLMCalls      int            `json:"llm_calls"`
	TokensIn      int            `json:"tokens_in"`
	TokensOut     int            `json:"tokens_out"`
	TotalCost     float64        `json:"total_cost"`
	Errors        []string       `json:"errors"`
	Success       bool           `json:"success"`
	TokensSaved   int            `json:"tokens_saved,omitempty"`
	CostSaved     float64        `json:"cost_saved,omitempty"`
}

// Observer is the interface for tracking agent activity
type Observer interface {
	// Emit records an event
	Emit(event Event)

	// Summary returns the execution summary
	Summary() *ExecutionSummary

	// Subscribe allows real-time event streaming (for LiveDashboard)
	Subscribe(ch chan<- Event)

	// Unsubscribe removes a subscriber
	Unsubscribe(ch chan<- Event)

	// SetVerbose enables real-time console output
	SetVerbose(verbose bool)
}

// AgentObserver is the concrete implementation of Observer
type AgentObserver struct {
	mu          sync.RWMutex
	startTime   time.Time
	events      []Event
	subscribers []chan<- Event
	verbose     bool

	// Aggregated stats
	filesCreated  map[string]int64 // path -> bytes
	filesModified map[string]int64
	filesDeleted  []string
	toolCalls     map[string]int
	llmCalls      int
	tokensIn      int
	tokensOut     int
	totalCost     float64
	errors        []string
	success       bool
	tokensSaved   int
	costSaved     float64
	activeModel   string
	activeBackend string
}

// NewObserver creates a new AgentObserver
func NewObserver() *AgentObserver {
	return &AgentObserver{
		startTime:     time.Now(),
		events:        make([]Event, 0),
		subscribers:   make([]chan<- Event, 0),
		filesCreated:  make(map[string]int64),
		filesModified: make(map[string]int64),
		filesDeleted:  make([]string, 0),
		toolCalls:     make(map[string]int),
		errors:        make([]string, 0),
		success:       true,
	}
}

// Emit records an event and notifies subscribers
func (o *AgentObserver) Emit(event Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.events = append(o.events, event)

	// Update aggregated stats based on event type
	switch e := event.(type) {
	case *ToolCallEvent:
		o.toolCalls[e.Name]++

		// If the tool event didn't calculate cost saved, let's do it based on active model
		if e.TokensSaved > 0 && e.CostSaved == 0 {
			costPerToken := 0.000015 // Default fallback (Claude 3.5 Sonnet level)
			if o.activeModel != "" {
				catalog := intelligence.NewModelCatalog()
				modelInfo := catalog.GetModelInfo(o.activeBackend, o.activeModel)
				costPerToken = modelInfo.CostPer1M / 1000000.0
			}
			e.CostSaved = float64(e.TokensSaved) * costPerToken
		}

		o.tokensSaved += e.TokensSaved
		o.costSaved += e.CostSaved

		if e.TokensSaved > 0 {
			o.recordSavingsToDisk(e.Name, e.TokensSaved, e.CostSaved)
		}

		if e.Error != "" {
			o.errors = append(o.errors, e.Error)
		}
	case *FileModifiedEvent:
		switch e.Operation {
		case "create":
			o.filesCreated[e.Path] = e.Bytes
		case "modify":
			o.filesModified[e.Path] = e.Bytes
		case "delete":
			o.filesDeleted = append(o.filesDeleted, e.Path)
		}
	case *LLMRequestEvent:
		o.llmCalls++
		o.tokensIn += e.TokensIn
		o.tokensOut += e.TokensOut
		o.totalCost += e.Cost
		o.activeModel = e.Model
		o.activeBackend = e.Backend
		if e.Error != "" {
			o.errors = append(o.errors, e.Error)
		}
	case *AgentEvent:
		if e.Phase == "end" && !e.Success {
			o.success = false
		}
	case *ValidationEvent:
		if !e.Success {
			o.errors = append(o.errors, e.Issues...)
		}
	}

	// Notify subscribers (non-blocking)
	for _, ch := range o.subscribers {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow
		}
	}

	// Verbose mode: print to console
	if o.verbose {
		o.printEvent(event)
	}
}

// printEvent outputs event details to console (verbose mode)
func (o *AgentObserver) printEvent(event Event) {
	switch e := event.(type) {
	case *ToolCallEvent:
		if e.Error != "" {
			fmt.Printf("  [TOOL] %-15s ERROR: %s\n", e.Name, e.Error)
		} else {
			fmt.Printf("  [TOOL] %-15s %.2fs\n", e.Name, e.Duration.Seconds())
		}
	case *FileModifiedEvent:
		switch e.Operation {
		case "create":
			fmt.Printf("  [FILE] + %-30s %s\n", e.Path, formatBytes(e.Bytes))
		case "modify":
			fmt.Printf("  [FILE] ~ %-30s %s\n", e.Path, formatBytes(e.Bytes))
		case "delete":
			fmt.Printf("  [FILE] - %s\n", e.Path)
		}
	case *LLMRequestEvent:
		costStr := fmt.Sprintf("$%.4f", e.Cost)
		if e.Cost == 0 {
			costStr = "$0.00"
		}
		fmt.Printf("  [LLM]  %-15s in:%s out:%s %s (%.2fs)\n",
			e.Model, formatNumber(e.TokensIn), formatNumber(e.TokensOut), costStr, e.Duration.Seconds())
	case *AgentEvent:
		if e.Phase == "start" {
			fmt.Printf("  [AGENT] %s started\n", e.Name)
		} else {
			status := "OK"
			if !e.Success {
				status = "FAILED"
			}
			fmt.Printf("  [AGENT] %s ended: %s\n", e.Name, status)
		}
	case *MovementEvent:
		if e.Phase == "start" {
			fmt.Printf("\n  >> Movement: %s\n", e.Name)
		} else {
			status := "OK"
			if !e.Success {
				status = "FAILED"
			}
			fmt.Printf("  << Movement complete: %s\n", status)
		}
	}
}

// Summary returns the execution summary
func (o *AgentObserver) Summary() *ExecutionSummary {
	o.mu.RLock()
	defer o.mu.RUnlock()

	created := make([]string, 0, len(o.filesCreated))
	for path := range o.filesCreated {
		created = append(created, path)
	}

	modified := make([]string, 0, len(o.filesModified))
	for path := range o.filesModified {
		modified = append(modified, path)
	}

	return &ExecutionSummary{
		Duration:      time.Since(o.startTime),
		FilesCreated:  created,
		FilesModified: modified,
		FilesDeleted:  o.filesDeleted,
		ToolCalls:     o.toolCalls,
		LLMCalls:      o.llmCalls,
		TokensIn:      o.tokensIn,
		TokensOut:     o.tokensOut,
		TotalCost:     o.totalCost,
		Errors:        o.errors,
		Success:       o.success && len(o.errors) == 0,
		TokensSaved:   o.tokensSaved,
		CostSaved:     o.costSaved,
	}
}

// Subscribe adds a channel to receive events
func (o *AgentObserver) Subscribe(ch chan<- Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.subscribers = append(o.subscribers, ch)
}

// Unsubscribe removes a subscriber
func (o *AgentObserver) Unsubscribe(ch chan<- Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, sub := range o.subscribers {
		if sub == ch {
			o.subscribers = append(o.subscribers[:i], o.subscribers[i+1:]...)
			break
		}
	}
}

// SetVerbose enables or disables real-time console output
func (o *AgentObserver) SetVerbose(verbose bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.verbose = verbose
}

// Events returns all recorded events
func (o *AgentObserver) Events() []Event {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return append([]Event{}, o.events...)
}

// GetFilesCreated returns the list of created files
func (o *AgentObserver) GetFilesCreated() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]string, 0, len(o.filesCreated))
	for path := range o.filesCreated {
		result = append(result, path)
	}
	return result
}

// GetFilesModified returns the list of modified files
func (o *AgentObserver) GetFilesModified() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]string, 0, len(o.filesModified))
	for path := range o.filesModified {
		result = append(result, path)
	}
	return result
}

// AllModifiedFiles returns created + modified files (for backward compat)
func (o *AgentObserver) AllModifiedFiles() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]string, 0, len(o.filesCreated)+len(o.filesModified))
	for path := range o.filesCreated {
		result = append(result, path)
	}
	for path := range o.filesModified {
		result = append(result, path)
	}
	return result
}

// PrintSummary outputs a professional formatted summary to console
func (o *AgentObserver) PrintSummary() {
	o.mu.RLock()
	defer o.mu.RUnlock()

	summary := o.Summary()

	// Header
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("                    EXECUTION SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	// Timing
	fmt.Println()
	fmt.Println("TIMING")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Total Duration:     %.2fs\n", summary.Duration.Seconds())

	// Calculate average per tool call if available
	totalToolCalls := 0
	for _, count := range summary.ToolCalls {
		totalToolCalls += count
	}
	if totalToolCalls > 0 {
		avgPerCall := summary.Duration.Seconds() / float64(totalToolCalls)
		fmt.Printf("  Avg per Tool Call:  %.2fs\n", avgPerCall)
	}

	// Files Section
	fmt.Println()
	fmt.Println("FILE CHANGES")
	fmt.Println(strings.Repeat("-", 40))

	totalFiles := len(o.filesCreated) + len(o.filesModified) + len(o.filesDeleted)
	var totalBytes int64
	for _, b := range o.filesCreated {
		totalBytes += b
	}
	for _, b := range o.filesModified {
		totalBytes += b
	}

	if totalFiles > 0 {
		fmt.Printf("  Files Created:      %d\n", len(o.filesCreated))
		fmt.Printf("  Files Modified:     %d\n", len(o.filesModified))
		fmt.Printf("  Files Deleted:      %d\n", len(o.filesDeleted))
		fmt.Printf("  Total Bytes:        %s\n", formatBytes(totalBytes))

		if len(o.filesCreated) > 0 {
			fmt.Println()
			fmt.Println("  Created:")
			for path, bytes := range o.filesCreated {
				fmt.Printf("    + %-35s %s\n", path, formatBytes(bytes))
			}
		}
		if len(o.filesModified) > 0 {
			fmt.Println()
			fmt.Println("  Modified:")
			for path, bytes := range o.filesModified {
				fmt.Printf("    ~ %-35s %s\n", path, formatBytes(bytes))
			}
		}
		if len(o.filesDeleted) > 0 {
			fmt.Println()
			fmt.Println("  Deleted:")
			for _, path := range o.filesDeleted {
				fmt.Printf("    - %s\n", path)
			}
		}
	} else {
		fmt.Println("  No files changed")
	}

	// Tool Calls Section
	fmt.Println()
	fmt.Println("TOOL USAGE")
	fmt.Println(strings.Repeat("-", 40))

	if totalToolCalls > 0 {
		fmt.Printf("  Total Calls:        %d\n", totalToolCalls)
		fmt.Println()
		fmt.Println("  Breakdown:")
		for tool, count := range summary.ToolCalls {
			pct := float64(count) / float64(totalToolCalls) * 100
			fmt.Printf("    %-20s %3d  (%5.1f%%)\n", tool, count, pct)
		}
	} else {
		fmt.Println("  No tool calls recorded")
	}

	// LLM Section
	fmt.Println()
	fmt.Println("LLM USAGE")
	fmt.Println(strings.Repeat("-", 40))

	if summary.LLMCalls > 0 || summary.TokensIn > 0 || summary.TokensOut > 0 {
		fmt.Printf("  API Calls:          %d\n", summary.LLMCalls)
		fmt.Printf("  Tokens In:          %s\n", formatNumber(summary.TokensIn))
		fmt.Printf("  Tokens Out:         %s\n", formatNumber(summary.TokensOut))
		fmt.Printf("  Total Tokens:       %s\n", formatNumber(summary.TokensIn+summary.TokensOut))
		fmt.Printf("  Total Cost:         $%.4f\n", summary.TotalCost)
		if summary.TokensSaved > 0 {
			fmt.Printf("  Tokens Saved (RTK): %s\n", formatNumber(summary.TokensSaved))
			fmt.Printf("  Cost Saved (RTK):   $%.4f\n", summary.CostSaved)
		}
	} else {
		fmt.Println("  No LLM calls recorded")
	}

	// Errors Section
	if len(summary.Errors) > 0 {
		fmt.Println()
		fmt.Println("ERRORS")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("  Count:              %d\n", len(summary.Errors))
		for i, err := range summary.Errors {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(summary.Errors)-5)
				break
			}
			// Truncate long errors
			if len(err) > 60 {
				err = err[:57] + "..."
			}
			fmt.Printf("  [%d] %s\n", i+1, err)
		}
	}

	// Final Status
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	if summary.Success {
		fmt.Println("  STATUS: SUCCESS")
	} else {
		fmt.Println("  STATUS: FAILED")
	}
	fmt.Println(strings.Repeat("=", 60))
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}

// formatNumber formats numbers with K/M suffixes
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

type SavingsRecord struct {
	Timestamp   string  `json:"timestamp"`
	Tool        string  `json:"tool"`
	Command     string  `json:"command"`
	TokensSaved int     `json:"tokens_saved"`
	CostSaved   float64 `json:"cost_saved"`
}

func (o *AgentObserver) recordSavingsToDisk(toolName string, tokensSaved int, costSaved float64) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	gptcodeDir := filepath.Join(home, ".gptcode")
	_ = os.MkdirAll(gptcodeDir, 0755)

	record := SavingsRecord{
		Timestamp:   time.Now().Format(time.RFC3339),
		Tool:        toolName,
		TokensSaved: tokensSaved,
		CostSaved:   costSaved,
	}

	// Try parsing command from Arguments if available in the last event
	if len(o.events) > 0 {
		if lastEvent, ok := o.events[len(o.events)-1].(*ToolCallEvent); ok {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(lastEvent.Arguments), &args); err == nil {
				if cmd, ok := args["command"].(string); ok {
					record.Command = cmd
				}
			}
		}
	}

	data, err := json.Marshal(record)
	if err != nil {
		return
	}

	savingsFile := filepath.Join(gptcodeDir, "savings.jsonl")
	f, err := os.OpenFile(savingsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(data, '\n'))
}

// GetLifetimeSavings parses the savings file and aggregates tokens and cost saved
func GetLifetimeSavings() (int, float64, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, 0, err
	}
	savingsFile := filepath.Join(home, ".gptcode", "savings.jsonl")
	file, err := os.Open(savingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	defer file.Close()

	var totalTokens int
	var totalCost float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record SavingsRecord
		if err := json.Unmarshal([]byte(scanner.Text()), &record); err == nil {
			totalTokens += record.TokensSaved
			totalCost += record.CostSaved
		}
	}
	return totalTokens, totalCost, scanner.Err()
}
