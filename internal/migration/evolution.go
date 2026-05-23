package migration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gptcode/internal/llm"
)

type EvolutionStep struct {
	Phase       int
	Description string
	SQL         string
	Rollback    string
	SafetyCheck string
}

type Evolution struct {
	Name  string
	Steps []EvolutionStep
}

type SchemaEvolution struct {
	provider llm.Provider
	model    string
	dir      string
}

func NewSchemaEvolution(provider llm.Provider, model, dir string) *SchemaEvolution {
	return &SchemaEvolution{
		provider: provider,
		model:    model,
		dir:      dir,
	}
}

func (s *SchemaEvolution) GenerateEvolution(ctx context.Context, description string) (*Evolution, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	prompt := fmt.Sprintf(`Generate a zero-downtime database migration strategy for: %s

Requirements:
1. Must be safe for production (no data loss)
2. Split into multiple deployment phases
3. Each phase must be independently deployable
4. Include rollback for each phase
5. Include safety checks

Return ONLY valid JSON in this format:
{
  "name": "descriptive_name",
  "steps": [
    {
      "phase": 1,
      "description": "Phase description",
      "sql": "SQL commands",
      "rollback": "Rollback SQL",
      "safety_check": "SELECT to verify"
    }
  ]
}

Example phases for adding a NOT NULL column:
- Phase 1: Add column as nullable
- Phase 2: Backfill data
- Phase 3: Add NOT NULL constraint

Be specific and safe.`, description)

	resp, err := s.provider.Chat(ctx, llm.ChatRequest{
		SystemPrompt: "You are a database expert that generates safe, zero-downtime migration strategies.",
		UserPrompt:   prompt,
		Model:        s.model,
	})

	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	evolution, err := s.parseEvolution(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse evolution: %w", err)
	}

	return evolution, nil
}

func (s *SchemaEvolution) parseEvolution(text string) (*Evolution, error) {
	cleaned := strings.TrimSpace(text)
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json\n")
		cleaned = strings.TrimSuffix(cleaned, "\n```")
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```\n")
		cleaned = strings.TrimSuffix(cleaned, "\n```")
	}

	var evolution Evolution
	if err := parseJSON(cleaned, &evolution); err != nil {
		return nil, err
	}

	if len(evolution.Steps) == 0 {
		return nil, fmt.Errorf("no steps generated")
	}

	return &evolution, nil
}

func parseJSON(jsonStr string, v interface{}) error {
	return fmt.Errorf("JSON parsing not implemented - simplified for demo")
}

func (s *SchemaEvolution) SaveEvolution(evolution *Evolution) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	timestamp := time.Now().Format("20060102150405")
	baseName := fmt.Sprintf("%s_%s", timestamp, evolution.Name)

	for _, step := range evolution.Steps {
		upFile := filepath.Join(s.dir, fmt.Sprintf("%s_phase%d_up.sql", baseName, step.Phase))
		downFile := filepath.Join(s.dir, fmt.Sprintf("%s_phase%d_down.sql", baseName, step.Phase))

		upContent := fmt.Sprintf("-- Phase %d: %s\n-- Safety check: %s\n\n%s\n",
			step.Phase, step.Description, step.SafetyCheck, step.SQL)

		downContent := fmt.Sprintf("-- Rollback Phase %d: %s\n\n%s\n",
			step.Phase, step.Description, step.Rollback)

		if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
			return fmt.Errorf("failed to write up migration: %w", err)
		}

		if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
			return fmt.Errorf("failed to write down migration: %w", err)
		}
	}

	readmePath := filepath.Join(s.dir, fmt.Sprintf("%s_README.md", baseName))
	readmeContent := s.generateReadme(evolution)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	return nil
}

func (s *SchemaEvolution) generateReadme(evolution *Evolution) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# %s\n\n", evolution.Name)
	sb.WriteString("## Zero-Downtime Migration Strategy\n\n")
	sb.WriteString("This migration is split into multiple phases for safe deployment:\n\n")

	for _, step := range evolution.Steps {
		fmt.Fprintf(&sb, "### Phase %d: %s\n\n", step.Phase, step.Description)
		fmt.Fprintf(&sb, "**Safety Check:**\n```sql\n%s\n```\n\n", step.SafetyCheck)
		fmt.Fprintf(&sb, "**Migration:**\n```sql\n%s\n```\n\n", step.SQL)
		fmt.Fprintf(&sb, "**Rollback:**\n```sql\n%s\n```\n\n", step.Rollback)
	}

	sb.WriteString("## Deployment Steps\n\n")
	for _, step := range evolution.Steps {
		fmt.Fprintf(&sb, "%d. Deploy Phase %d migration\n", step.Phase, step.Phase)
		sb.WriteString("   - Run safety check\n")
		sb.WriteString("   - Apply migration\n")
		sb.WriteString("   - Verify no errors\n")
		sb.WriteString("   - Deploy application code (if needed)\n\n")
	}

	return sb.String()
}

func (s *SchemaEvolution) ValidateMigration(sqlFile string) error {
	content, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	sql := string(content)

	unsafePatterns := []string{
		"DROP TABLE",
		"DROP COLUMN",
		"ALTER COLUMN.*SET NOT NULL",
		"TRUNCATE",
	}

	for _, pattern := range unsafePatterns {
		if strings.Contains(strings.ToUpper(sql), pattern) {
			return fmt.Errorf("potentially unsafe operation detected: %s", pattern)
		}
	}

	return nil
}

func (s *SchemaEvolution) TestMigration(sqlFile string, dbURL string) error {
	content, err := os.ReadFile(sqlFile)
	if err != nil {
		return err
	}

	tmpDB := fmt.Sprintf("%s_test_%d", dbURL, time.Now().Unix())

	createCmd := exec.Command("createdb", tmpDB)
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create test database: %w", err)
	}

	defer exec.Command("dropdb", tmpDB).Run()

	psqlCmd := exec.Command("psql", tmpDB)
	psqlCmd.Stdin = strings.NewReader(string(content))
	output, err := psqlCmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("migration failed: %s", string(output))
	}

	return nil
}
