package autonomous

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jadercorrea/gptcode/internal/maestro"
)

type AutoEditor struct {
	maestro  *maestro.Conductor
	healer   *SelfHealer
	retry    *SmartRetry
	learning *LearningEngine
	context  *ContextManager
	cwd      string
}

func NewAutoEditor(cwd string) *AutoEditor {
	return &AutoEditor{
		healer:   NewSelfHealer(cwd),
		retry:    NewSmartRetry(3),
		learning: NewLearningEngine(cwd),
		context:  NewContextManager(8000),
		cwd:      cwd,
	}
}

func (ae *AutoEditor) SetMaestro(maestro *maestro.Conductor) {
	ae.maestro = maestro
}

type EditResult struct {
	Success   bool
	Files     []string
	Retries   int
	Error     error
	Recovered bool
	Lessions  []string
}

func (ae *AutoEditor) Execute(ctx context.Context, task string) *EditResult {
	result := &EditResult{}

	fmt.Printf("[AUTO-EDITOR] Starting autonomous edit...\n")

	// Get learned suggestions
	if suggestions := ae.learning.GetSuggestions(task); len(suggestions) > 0 {
		fmt.Printf("[AUTO-EDITOR] Learned suggestions: %s\n", suggestions[0])
	}

	// Execute with retry
	retryResult, err := ae.retry.Execute(ctx, task, func() error {
		return ae.executeOnce(ctx, task, result)
	})

	if err != nil {
		result.Error = err
		result.Retries = retryResult.Attempts
		return result
	}

	result.Success = true
	result.Retries = retryResult.Attempts
	result.Lessions = ae.learning.GetSuggestions(task)

	// Record success for learning
	ae.learning.RecordAttempt(task, []string{"execute"}, true, "")

	return result
}

func (ae *AutoEditor) executeOnce(ctx context.Context, task string, result *EditResult) error {
	// Execute via maestro
	err := ae.maestro.ExecuteTask(ctx, task, "medium")
	if err == nil {
		result.Success = true
		return nil
	}

	// Try self-healing
	healResult := ae.healer.AnalyzeAndHeal(err.Error())
	if healResult.Fixed {
		result.Recovered = true
		fmt.Printf("[AUTO-EDITOR] Self-healed: %s\n", healResult.Action)
		return nil
	}

	// Get recovery tip
	classifier := NewErrorClassifier()
	category := classifier.Classify(err)
	fmt.Printf("[AUTO-EDITOR] Error: %s - %s\n", category.Category, category.Tip)

	// Record failure for learning
	ae.learning.RecordAttempt(task, []string{"execute"}, false, category.Tip)

	return err
}

func (ae *AutoEditor) GetStats() string {
	return fmt.Sprintf("Learning: %s, Context: %s",
		ae.learning.GetStats(),
		ae.context.GetStats())
}

type MultiEditor struct {
	editors     []*AutoEditor
	concurrency int
}

func NewMultiEditor(concurrency int) *MultiEditor {
	return &MultiEditor{
		concurrency: concurrency,
		editors:     make([]*AutoEditor, concurrency),
	}
}

func (me *MultiEditor) AddEditor(cwd string) *AutoEditor {
	editor := NewAutoEditor(cwd)
	me.editors = append(me.editors, editor)
	return editor
}

func (me *MultiEditor) ExecuteAll(ctx context.Context, tasks []string) *MultiEditResult {
	result := &MultiEditResult{
		StartTime: time.Now(),
	}

	sem := make(chan struct{}, me.concurrency)
	taskCh := make(chan string, len(tasks))

	for _, task := range tasks {
		taskCh <- task
	}
	close(taskCh)

	for i, task := range tasks {
		sem <- struct{}{}
		go func(idx int, t string) {
			editor := me.editors[idx%len(me.editors)]
			editResult := editor.Execute(ctx, t)
			result.mu.Lock()
			result.Results = append(result.Results, *editResult)
			if editResult.Success {
				result.Completed++
			} else {
				result.Failed++
			}
			result.mu.Unlock()
			<-sem
		}(i, task)
	}

	result.Duration = time.Since(result.StartTime)
	return result
}

type MultiEditResult struct {
	Results   []EditResult
	Completed int
	Failed    int
	Duration  time.Duration
	StartTime time.Time
	mu        sync.Mutex
}

func (r *MultiEditResult) Summary() string {
	return fmt.Sprintf("Completed: %d, Failed: %d, Duration: %v, Success Rate: %.0f%%",
		r.Completed, r.Failed, r.Duration, r.SuccessRate())
}

func (r *MultiEditResult) SuccessRate() float64 {
	total := r.Completed + r.Failed
	if total == 0 {
		return 0
	}
	return float64(r.Completed) / float64(total) * 100
}
