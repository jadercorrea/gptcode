package prompt

import (
	"os"
	"strings"
	"testing"
)

func TestBuilder_CavemanIntegration(t *testing.T) {
	builder := &Builder{
		SystemPath:   "",
		ProfilePath:  "",
		Store:        nil,
		SkillsLoader: NewSkillsLoader(),
	}

	opts := BuildOptions{
		Lang: "go",
		Mode: "chat",
		Task: "write some code",
	}

	t.Run("caveman mode off", func(t *testing.T) {
		os.Setenv("CAVEMAN_MODE", "off")
		defer os.Unsetenv("CAVEMAN_MODE")

		prompt := builder.BuildSystemPrompt(opts)
		if strings.Contains(prompt, "Caveman Optimization Mode") {
			t.Error("Expected prompt not to contain Caveman instructions when mode is off")
		}
	})

	t.Run("caveman mode full", func(t *testing.T) {
		os.Setenv("CAVEMAN_MODE", "full")
		defer os.Unsetenv("CAVEMAN_MODE")

		prompt := builder.BuildSystemPrompt(opts)
		if !strings.Contains(prompt, "Caveman Optimization Mode") {
			t.Error("Expected prompt to contain Caveman instructions when mode is full")
		}
		if !strings.Contains(prompt, "Intensity Level: full") {
			t.Error("Expected prompt to specify Intensity Level: full")
		}
		if !strings.Contains(prompt, "Classic caveman") {
			t.Error("Expected prompt to contain description for level: full")
		}
	})

	t.Run("caveman mode ultra", func(t *testing.T) {
		os.Setenv("CAVEMAN_MODE", "ultra")
		defer os.Unsetenv("CAVEMAN_MODE")

		prompt := builder.BuildSystemPrompt(opts)
		if !strings.Contains(prompt, "Caveman Optimization Mode") {
			t.Error("Expected prompt to contain Caveman instructions when mode is ultra")
		}
		if !strings.Contains(prompt, "Intensity Level: ultra") {
			t.Error("Expected prompt to specify Intensity Level: ultra")
		}
	})
}
