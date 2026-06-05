package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gptcode/internal/config"
)

type BuildOptions struct {
	Lang string
	Mode string
	Hint string
	Task string // Task description for product skill detection
}

type Builder struct {
	ProfilePath  string
	SystemPath   string
	Store        MemoryStore
	SkillsLoader *SkillsLoader
}

func NewDefaultBuilder(store MemoryStore) *Builder {
	home, _ := os.UserHomeDir()
	profile := filepath.Join(home, ".gptcode", "profile.yaml")
	system := filepath.Join(home, ".gptcode", "system_prompt.md")
	return &Builder{
		ProfilePath:  profile,
		SystemPath:   system,
		Store:        store,
		SkillsLoader: NewSkillsLoader(),
	}
}

func (b *Builder) BuildSystemPrompt(opts BuildOptions) string {
	base := mustReadFile(b.SystemPath)
	profile := mustReadFile(b.ProfilePath)
	mem := ""
	if b.Store != nil {
		mem = b.Store.LastRelevant(opts.Lang)
	}

	// Build skills section with multiple skills
	skillsSection := ""
	if b.SkillsLoader != nil {
		var skillContents []string

		// 1. Load language-specific skill
		if opts.Lang != "" {
			langSkill := b.SkillsLoader.LoadForLanguage(opts.Lang)
			if langSkill != "" {
				skillContents = append(skillContents, fmt.Sprintf("## Language: %s\n\n%s", opts.Lang, langSkill))
			}
		}

		// 2. Load product skills based on task keywords
		productSkills := b.SkillsLoader.LoadProductSkillsForTask(opts.Task)
		skillContents = append(skillContents, productSkills...)

		// 3. For autonomous mode, always include production-ready skill
		if opts.Mode == "autonomous" || opts.Mode == "do" {
			prodSkill := b.SkillsLoader.LoadByName("production-ready")
			if prodSkill != "" {
				// Check if not already added
				alreadyAdded := false
				for _, s := range skillContents {
					if len(s) > 100 && s[:100] == prodSkill[:min(100, len(prodSkill))] {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					skillContents = append(skillContents, prodSkill)
				}
			}
		}

		// 4. Load Caveman mode if enabled
		cavemanMode := os.Getenv("CAVEMAN_MODE")
		if cavemanMode == "" {
			setup, err := config.LoadSetup()
			if err == nil {
				cavemanMode = setup.Defaults.CavemanMode
			}
		}
		if cavemanMode != "" && cavemanMode != "off" {
			cavemanSkill := b.SkillsLoader.LoadByName("caveman")
			if cavemanSkill != "" {
				rendered := renderTemplate(cavemanSkill, map[string]interface{}{
					"Level": cavemanMode,
				})
				if rendered != "" {
					skillContents = append(skillContents, fmt.Sprintf("## Caveman Optimization Mode\n\n%s", rendered))
				}
			}
		}

		// Combine all skills
		if len(skillContents) > 0 {
			combined := ""
			for i, skill := range skillContents {
				if i > 0 {
					combined += "\n\n---\n\n"
				}
				combined += skill
			}
			skillsSection = fmt.Sprintf(`
---

# Product Engineering Skills

%s
`, combined)
		}
	}

	return fmt.Sprintf(`%s

---

# GPTCode Profile (YAML)

%s

---

# Relevant Memory

%s
%s
---

# Current Session Context

Language: %s
Mode: %s
Task: %s
Hint: %s
`, base, profile, mem, skillsSection, opts.Lang, opts.Mode, opts.Task, opts.Hint)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mustReadFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

type MemoryStore interface {
	LastRelevant(lang string) string
}

func renderTemplate(tmplStr string, data interface{}) string {
	tmpl, err := template.New("tmpl").Parse(tmplStr)
	if err != nil {
		return tmplStr
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return tmplStr
	}
	return buf.String()
}
