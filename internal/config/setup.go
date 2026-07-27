package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jadercorrea/gptcode/internal/langdetect"
	"gopkg.in/yaml.v3"
)

func RunSetup() {
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".gptcode")

	if err := os.MkdirAll(target, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: failed to create ~/.gptcode:", err)
		return
	}

	templateDir := detectTemplateDir()

	copyIfMissing(templateDir, target, "profile.yaml")
	copyIfMissing(templateDir, target, "system_prompt.md")

	setupPath := filepath.Join(target, "setup.yaml")
	if _, err := os.Stat(setupPath); err == nil {
		fmt.Fprintln(os.Stderr, "\nsetup.yaml already exists.")
		fmt.Fprint(os.Stderr, "Reconfigure? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
			fmt.Fprintln(os.Stderr, "GPTCode: setup complete → ~/.gptcode")
			return
		}
	}

	setup := interactiveSetup()
	if err := saveSetup(setupPath, setup); err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: failed to save setup.yaml:", err)
		return
	}

	fmt.Fprintln(os.Stderr, "\nGPTCode: setup complete → ~/.gptcode")
}

func RunSetupQuickStart() {
	home, _ := os.UserHomeDir()
	target := filepath.Join(home, ".gptcode")

	if err := os.MkdirAll(target, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: failed to create ~/.gptcode:", err)
		return
	}

	templateDir := detectTemplateDir()

	copyIfMissing(templateDir, target, "profile.yaml")
	copyIfMissing(templateDir, target, "system_prompt.md")

	setupPath := filepath.Join(target, "setup.yaml")

	lang := langdetect.DetectLanguage(".")
	setup := &Setup{
		Defaults: struct {
			Mode               string  `yaml:"mode,omitempty"`
			Backend            string  `yaml:"backend"`
			Profile            string  `yaml:"profile,omitempty"`
			Model              string  `yaml:"model,omitempty"`
			Lang               string  `yaml:"lang"`
			SystemPromptFile   string  `yaml:"system_prompt_file,omitempty"`
			MLComplexThreshold float64 `yaml:"ml_complex_threshold,omitempty"`
			MLIntentThreshold  float64 `yaml:"ml_intent_threshold,omitempty"`
			GraphMaxFiles      int     `yaml:"graph_max_files,omitempty"`
			BudgetMode         bool    `yaml:"budget_mode,omitempty"`
			MaxCostPerTask     float64 `yaml:"max_cost_per_task,omitempty"`
			MonthlyBudget      float64 `yaml:"monthly_budget,omitempty"`
			CavemanMode        string  `yaml:"caveman_mode,omitempty"`
		}{
			Backend: "openrouter",
			Model:   "free",
			Lang:    strings.ToLower(string(lang)),
		},
		Backend: map[string]BackendConfig{
			"openrouter": {
				Type:         "openai",
				BaseURL:      "https://openrouter.ai/api/v1",
				DefaultModel: "stepfun/step-3.5-flash:free",
				Models: map[string]string{
					"free": "stepfun/step-3.5-flash:free",
				},
			},
		},
	}

	if err := saveSetup(setupPath, setup); err != nil {
		fmt.Fprintln(os.Stderr, "GPTCode: failed to save setup.yaml:", err)
		return
	}

	fmt.Print(`
╔════════════════════════════════════════════════════════════╗
║              Quick Start Setup Complete!                 ║
╠════════════════════════════════════════════════════════════╣
║                                                            ║
║  Next steps:                                             ║
║                                                            ║
║  1. Get a free API key: https://openrouter.ai/keys     ║
║  2. Run: gt key openrouter                              ║
║  3. Run: gt go "hello" or gt run "listar arquivos"     ║
║                                                            ║
║  Optional - Enable web search:                           ║
║    gt key tavily      # Get key from https://tavily.com  ║
║    gt key exa         # Get key from https://exa.ai      ║
║                                                            ╚════════════════════════════════════════════════════╝
`)

	installFeedbackHook()
}

func installFeedbackHook() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	hookDir := filepath.Join(home, ".gptcode")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return
	}

	shell := os.Getenv("SHELL")
	isZsh := strings.Contains(shell, "zsh")

	var hookPath string
	var hookContent string
	var sourceLine string

	if isZsh {
		hookPath = filepath.Join(hookDir, "feedback_hook.zsh")
		hookContent = `chu_mark_suggestion_widget() {
	local f="$HOME/.gptcode/last_suggestion_cmd"
	print -r -- "$BUFFER" > "$f"
	zle -M "Suggestion captured"
}

zle -N chu_mark_suggestion_widget
bindkey -M emacs "^G" chu_mark_suggestion_widget
bindkey -M viins "^G" chu_mark_suggestion_widget

preexec_chu_feedback() {
	local cmd="$1"
	local sfile="$HOME/.gptcode/last_suggestion_cmd"
	if [[ -f "$sfile" ]]; then
		print -r -- "$(<"$sfile")" > "$HOME/.gptcode/.pending_wrong"
		print -r -- "$cmd" > "$HOME/.gptcode/.pending_correct"
	fi
}

precmd_chu_feedback() {
	local wrongf="$HOME/.gptcode/.pending_wrong"
	local correctf="$HOME/.gptcode/.pending_correct"
	if [[ -f "$wrongf" && -f "$correctf" ]]; then
		local wrong="$(<"$wrongf")"
		local correct="$(<"$correctf")"
		local files=""
		if command -v git >/dev/null 2>&1; then
			if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
				files=$(git diff --name-only)
			fi
		fi
		local -a args
		args=(gt feedback submit --sentiment=bad --kind=command --source=shell --agent=editor --wrong="$wrong" --correct="$correct")
		
		if [[ -n "$files" ]]; then
			local f
			for f in ${(f)files}; do
				args+=(--files "$f")
			done
		fi
		gt $args >/dev/null 2>&1
		rm -f "$wrongf" "$correctf" "$HOME/.gptcode/last_suggestion_cmd"
	fi
}

autoload -Uz add-zsh-hook
add-zsh-hook preexec preexec_chu_feedback
add-zsh-hook precmd precmd_chu_feedback
`
		sourceLine = "source $HOME/.gptcode/feedback_hook.zsh"
	} else {
		hookPath = filepath.Join(hookDir, "feedback_hook.bash")
		hookContent = `# GPTCode feedback hook for bash
# Tracks command corrections for learning
PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND;} gt feedback watch"
`
		sourceLine = ". \"$HOME/.gptcode/feedback_hook.bash\""
	}

	os.WriteFile(hookPath, []byte(hookContent), 0644)

	rcPath := filepath.Join(home, ".zshrc")
	if _, err := os.Stat(rcPath); err != nil {
		rcPath = filepath.Join(home, ".bashrc")
	}

	if existing, err := os.ReadFile(rcPath); err == nil {
		if !strings.Contains(string(existing), "feedback_hook") {
			f, _ := os.OpenFile(rcPath, os.O_APPEND|os.O_WRONLY, 0644)
			defer f.Close()
			f.WriteString("\n# GPTCode feedback hook\n")
			f.WriteString(sourceLine + "\n")
		}
	}
}

func detectTemplateDir() string {
	if env := os.Getenv("GPTCODE_TEMPLATES_DIR"); env != "" {
		return env
	}
	if _, err := os.Stat("internal/prompt/templates"); err == nil {
		return "internal/prompt/templates"
	}
	return "templates"
}

func LoadSetup() (*Setup, error) {
	path := filepath.Join(configDir(), "setup.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return &Setup{}, err
	}
	var s Setup
	if err := yaml.Unmarshal(b, &s); err != nil {
		return &Setup{}, err
	}
	return &s, nil
}

func SaveSetup(setup *Setup) error {
	path := filepath.Join(configDir(), "setup.yaml")
	return saveSetup(path, setup)
}

func interactiveSetup() *Setup {
	reader := bufio.NewReader(os.Stdin)
	setup := &Setup{
		Backend: make(map[string]BackendConfig),
	}

	fmt.Fprintln(os.Stderr, "\n=== GPTCode Setup ===")
	fmt.Fprintln(os.Stderr, "\nChoose your setup:")
	fmt.Fprintln(os.Stderr, "1) Quick Start (recommended) - Free, no API key needed")
	fmt.Fprintln(os.Stderr, "2) Local (Ollama) - Run models on your machine")
	fmt.Fprintln(os.Stderr, "3) Cloud API (OpenAI, Groq, etc) - Use external APIs")
	fmt.Fprintln(os.Stderr, "4) Both Local and Cloud")
	fmt.Fprint(os.Stderr, "\nChoice (1-4): ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	// Quick Start: Configure OpenRouter with free model automatically
	if choice == "1" {
		fmt.Fprintln(os.Stderr, "\n--- Quick Start: OpenRouter Free ---")
		fmt.Fprintln(os.Stderr, "Using stepfun/step-3.5-flash:free (no cost, supports tools)")

		// Ask for API key
		fmt.Fprintln(os.Stderr, "\nTo get started, you need a free API key from OpenRouter.")
		fmt.Fprintln(os.Stderr, "Get one at: https://openrouter.ai/keys")
		fmt.Fprint(os.Stderr, "\nPaste your OpenRouter API key (or press Enter to skip): ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)

		setup.Backend["openrouter"] = BackendConfig{
			Type:         "openai",
			BaseURL:      "https://openrouter.ai/api/v1",
			DefaultModel: "stepfun/step-3.5-flash:free",
			Models: map[string]string{
				"free": "stepfun/step-3.5-flash:free",
			},
		}
		setup.Defaults.Backend = "openrouter"
		setup.Defaults.Model = "free"
		lang := langdetect.DetectLanguage(".")
		setup.Defaults.Lang = strings.ToLower(string(lang))

		// Save API key if provided
		if apiKey != "" {
			envVar := "OPENROUTER_API_KEY"
			os.Setenv(envVar, apiKey)
			if err := saveAPIKeyToProfile(envVar, apiKey); err != nil {
				fmt.Fprintf(os.Stderr, "\nWarning: Could not save API key: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "\n✓ API key saved!")
			}
		} else {
			fmt.Fprintln(os.Stderr, "\n⚠️  No API key provided.")
			fmt.Fprintln(os.Stderr, "To enable, run: gt key openrouter")
		}

		return setup
	}

	useLocal := choice == "2" || choice == "4"
	useAPI := choice == "3" || choice == "4"

	if useLocal {
		fmt.Fprintln(os.Stderr, "\n--- Ollama (Local) ---")
		fmt.Fprint(os.Stderr, "Base URL [http://localhost:11434]: ")
		baseURL, _ := reader.ReadString('\n')
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}

		for {
			fmt.Fprintln(os.Stderr, "\nModels (one or more, comma-separated):")
			fmt.Fprintln(os.Stderr, "  Examples: qwen3-coder,gpt-oss")
			fmt.Fprint(os.Stderr, "Models: ")
			modelsInput, _ := reader.ReadString('\n')
			modelsInput = strings.TrimSpace(modelsInput)
			if modelsInput == "" {
				fmt.Fprintln(os.Stderr, "At least one model is required")
				continue
			}

			modelsList := strings.Split(modelsInput, ",")
			modelsMap := make(map[string]string)
			for _, m := range modelsList {
				m = strings.TrimSpace(m)
				if m != "" {
					modelsMap[m] = m
				}
			}

			defaultModel := ""
			if len(modelsList) > 0 {
				defaultModel = strings.TrimSpace(modelsList[0])
			}

			setup.Backend["ollama"] = BackendConfig{
				Type:         "ollama",
				BaseURL:      baseURL,
				DefaultModel: defaultModel,
				Models:       modelsMap,
			}
			break
		}
	}

	if useAPI {
		for {
			fmt.Fprintln(os.Stderr, "\n--- OpenAI-compatible API Service ---")
			fmt.Fprintln(os.Stderr, "Examples: groq, openrouter, openai, deepseek, deepinfra")
			fmt.Fprint(os.Stderr, "\nService name (empty to finish): ")
			backendName, _ := reader.ReadString('\n')
			backendName = strings.TrimSpace(backendName)
			if backendName == "" {
				break
			}

			knownURLs := map[string]string{
				"groq":       "https://api.groq.com/openai/v1",
				"openrouter": "https://openrouter.ai/api/v1",
				"openai":     "https://api.openai.com/v1",
				"deepseek":   "https://api.deepseek.com/v1",
				"deepinfra":  "https://api.deepinfra.com/v1/openai",
			}

			defaultURL := knownURLs[backendName]
			if defaultURL != "" {
				fmt.Fprintf(os.Stderr, "Base URL [%s]: ", defaultURL)
			} else {
				fmt.Fprint(os.Stderr, "Base URL: ")
			}
			baseURL, _ := reader.ReadString('\n')
			baseURL = strings.TrimSpace(baseURL)
			if baseURL == "" {
				if defaultURL != "" {
					baseURL = defaultURL
				} else {
					fmt.Fprintln(os.Stderr, "Base URL is required, skipping...")
					continue
				}
			}

			fmt.Fprint(os.Stderr, "API Key: ")
			apiKey, _ := reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)

			fmt.Fprintln(os.Stderr, "\nModels (one or more, comma-separated):")
			fmt.Fprintln(os.Stderr, "  Example for Groq: llama-3.3-70b-versatile,llama-3.1-8b-instant")
			fmt.Fprintln(os.Stderr, "  Example for OpenRouter: kwaipilot/kat-coder-pro")
			fmt.Fprint(os.Stderr, "Models: ")
			modelsInput, _ := reader.ReadString('\n')
			modelsInput = strings.TrimSpace(modelsInput)
			if modelsInput == "" {
				fmt.Fprintln(os.Stderr, "At least one model is required, skipping...")
				continue
			}

			modelsList := strings.Split(modelsInput, ",")
			modelsMap := make(map[string]string)
			for _, m := range modelsList {
				m = strings.TrimSpace(m)
				if m != "" {
					modelsMap[m] = m
				}
			}

			defaultModel := ""
			if len(modelsList) > 0 {
				defaultModel = strings.TrimSpace(modelsList[0])
			}

			setup.Backend[backendName] = BackendConfig{
				Type:         "openai",
				BaseURL:      baseURL,
				DefaultModel: defaultModel,
				Models:       modelsMap,
			}

			if apiKey != "" {
				envVar := strings.ToUpper(backendName) + "_API_KEY"
				os.Setenv(envVar, apiKey)

				if err := saveAPIKeyToProfile(envVar, apiKey); err != nil {
					fmt.Fprintf(os.Stderr, "\nWarning: Could not auto-save to shell profile: %v\n", err)
					fmt.Fprintf(os.Stderr, "Manually add: export %s=%s\n", envVar, apiKey)
				} else {
					fmt.Fprintf(os.Stderr, "\n✓ API key saved to shell profile\n")
					fmt.Fprintf(os.Stderr, "  Restart your terminal or run: source ~/.zshrc\n")
				}
			}
		}
	}

	fmt.Fprintln(os.Stderr, "\n--- Defaults ---")
	availableBackends := []string{}
	for name := range setup.Backend {
		availableBackends = append(availableBackends, name)
	}
	defaultBackend := ""
	if len(availableBackends) > 0 {
		defaultBackend = availableBackends[0]
	}
	fmt.Fprintf(os.Stderr, "Available backends: %s\n", strings.Join(availableBackends, ", "))
	fmt.Fprintf(os.Stderr, "Default backend [%s]: ", defaultBackend)
	backend, _ := reader.ReadString('\n')
	backend = strings.TrimSpace(backend)
	if backend == "" {
		backend = defaultBackend
	}
	setup.Defaults.Backend = backend

	defaultModel := ""
	if cfg, ok := setup.Backend[backend]; ok {
		defaultModel = cfg.DefaultModel
	}
	fmt.Fprintf(os.Stderr, "Default model [%s]: ", defaultModel)
	model, _ := reader.ReadString('\n')
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}
	setup.Defaults.Model = model

	fmt.Fprint(os.Stderr, "Default language [go]: ")
	lang, _ := reader.ReadString('\n')
	lang = strings.TrimSpace(lang)
	if lang == "" {
		lang = "go"
	}
	setup.Defaults.Lang = lang

	return setup
}

func saveSetup(path string, setup *Setup) error {
	data, err := yaml.Marshal(setup)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func GetAPIKey(backendName string) string {
	// keys.yaml takes priority over env vars
	home, err := os.UserHomeDir()
	if err == nil {
		keysPath := filepath.Join(home, ".gptcode", "keys.yaml")
		if data, err := os.ReadFile(keysPath); err == nil {
			var keys map[string]string
			if err := yaml.Unmarshal(data, &keys); err == nil {
				if val, ok := keys[backendName]; ok && val != "" {
					return val
				}
			}
		}
	}

	// Fallback to env var
	envVar := strings.ToUpper(backendName) + "_API_KEY"
	return os.Getenv(envVar)
}

func LoadAPIKey(keyName string) string {
	// keys.yaml takes priority over env vars
	home, err := os.UserHomeDir()
	if err == nil {
		keysPath := filepath.Join(home, ".gptcode", "keys.yaml")
		if data, err := os.ReadFile(keysPath); err == nil {
			var keys map[string]string
			if err := yaml.Unmarshal(data, &keys); err == nil {
				// Try exact match first
				if val, ok := keys[keyName]; ok && val != "" {
					return val
				}
				// Try without _API_KEY suffix
				cleanName := strings.TrimSuffix(keyName, "_API_KEY")
				if val, ok := keys[cleanName]; ok && val != "" {
					return val
				}
			}
		}
	}

	// Fallback to env var
	return os.Getenv(keyName)
}

func saveAPIKeyToKeysFile(backendName, apiKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	keysPath := filepath.Join(home, ".gptcode", "keys.yaml")

	keys := make(map[string]string)
	if data, err := os.ReadFile(keysPath); err == nil {
		_ = yaml.Unmarshal(data, &keys)
	}

	keys[backendName] = apiKey

	data, err := yaml.Marshal(keys)
	if err != nil {
		return err
	}

	return os.WriteFile(keysPath, data, 0o600)
}

func UpdateAPIKey(backendName string) error {
	// Handle web search API keys specially (they're not backends)
	lowerName := strings.ToLower(backendName)
	if lowerName == "tavily" || lowerName == "exa" {
		return updateSearchAPIKey(backendName)
	}

	setup, err := LoadSetup()
	if err != nil {
		return fmt.Errorf("could not load setup: %w", err)
	}

	if _, ok := setup.Backend[backendName]; !ok {
		return fmt.Errorf("backend %q not found in setup. Available: %v", backendName, getBackendNames(setup))
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "Enter API key for %s: ", backendName)
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if err := saveAPIKeyToKeysFile(backendName, apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n✓ API key saved to ~/.gptcode/keys.yaml\n")
	fmt.Fprintf(os.Stderr, "  (with 0600 permissions for security)\n")

	return nil
}

func updateSearchAPIKey(name string) error {
	envVar := strings.ToUpper(name) + "_API_KEY"

	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "Enter API key for %s (will be saved as %s): ", name, envVar)
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if err := saveAPIKeyToKeysFile(envVar, apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\n✓ API key saved to ~/.gptcode/keys.yaml\n")
	fmt.Fprintf(os.Stderr, "  (with 0600 permissions for security)\n")
	fmt.Fprintf(os.Stderr, "  Env var: %s\n", envVar)

	return nil
}

func getBackendNames(setup *Setup) []string {
	names := make([]string, 0, len(setup.Backend))
	for name := range setup.Backend {
		names = append(names, name)
	}
	return names
}

func saveAPIKeyToProfile(envVar, apiKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	shell := os.Getenv("SHELL")
	var profilePath string

	if strings.Contains(shell, "zsh") {
		profilePath = filepath.Join(home, ".zshrc")
	} else if strings.Contains(shell, "bash") {
		profilePath = filepath.Join(home, ".bashrc")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			profilePath = filepath.Join(home, ".bash_profile")
		}
	} else {
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	exportLine := fmt.Sprintf("export %s=%q\n", envVar, apiKey)

	if _, err := os.Stat(profilePath); err == nil {
		content, err := os.ReadFile(profilePath)
		if err != nil {
			return err
		}

		if strings.Contains(string(content), envVar) {
			return fmt.Errorf("%s already exists in %s", envVar, profilePath)
		}
	}

	f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString("\n# GPTCode API key\n" + exportLine); err != nil {
		return err
	}

	return nil
}

func copyIfMissing(srcDir, dstDir, file string) {
	src := filepath.Join(srcDir, file)
	dst := filepath.Join(dstDir, file)

	if _, err := os.Stat(dst); err == nil {
		fmt.Fprintln(os.Stderr, "keeping existing", dst)
		return
	}

	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read template", src, ":", err)
		return
	}

	if err := os.WriteFile(dst, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "could not write", dst, ":", err)
		return
	}

	fmt.Fprintln(os.Stderr, "wrote", dst)
}
