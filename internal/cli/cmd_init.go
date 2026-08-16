package cli

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

//go:embed project.yaml.tmpl
var projectTemplate string

var (
	initInteractive bool
	initGenre       string
	initPremise     string
	initPOV         string
	initLanguage    string
	initProtagonist string
	initProvider    string
	initModel       string
	initAPIKey      string
)

var initCmd = &cobra.Command{
	Use:   "init <project-name>",
	Short: "Initialize a new Fiction Factory project",
	Long: `Create a new Fiction Factory project with the standard directory structure.

This creates:
  - project.yaml       (project configuration + LLM settings)
  - world.yaml         (world state: facts, threads, timeline)
  - characters/protagonist.yaml (protagonist definition)
  - timeline/          (event storage)
  - generated/         (exported chapter markdown files)

Examples:
  ff init my-novel
  ff init --interactive my-novel
  ff init --genre fantasy --premise "A dying world" my-novel`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Run interactive setup wizard")
	initCmd.Flags().StringVar(&initGenre, "genre", "", "Story genre (e.g., fantasy, sci-fi)")
	initCmd.Flags().StringVar(&initPremise, "premise", "", "Story premise / one-line summary")
	initCmd.Flags().StringVar(&initPOV, "pov", "third_person_limited", "Point of view: first_person | third_person_limited")
	initCmd.Flags().StringVar(&initLanguage, "language", "zh", "Novel language: zh | en")
	initCmd.Flags().StringVar(&initProtagonist, "protagonist", "Protagonist", "Protagonist name")
	initCmd.Flags().StringVar(&initProvider, "provider", "deepseek", "LLM provider: deepseek | claude | ollama")
	initCmd.Flags().StringVar(&initModel, "model", "deepseek-chat", "LLM model name")
	initCmd.Flags().StringVar(&initAPIKey, "api-key", "", "LLM API key (or set env var)")
}

// templateData holds all values needed to scaffold a project.
type templateData struct {
	Name            string
	CreatedAt       string
	StoryTitle      string
	Genre           string
	Premise         string
	POV             string
	Tense           string
	Language        string
	ProtagonistName string
	InitialBeliefs  []string
	InitialGoals    []string
	InitialFears    []string
	InitialValues   []string
	LLMProvider     string
	LLMModel        string
	LLMAPIKey       string
	LLMBaseURL      string
	LLMTemperature  float64
	LLMMaxTokens    int
}

func runInit(cmd *cobra.Command, args []string) error {
	projectName := args[0]
	baseDir := GetProjectDir()
	var projectPath string
	if filepath.IsAbs(projectName) {
		projectPath = projectName
	} else {
		projectPath = filepath.Join(baseDir, projectName)
	}

	// Check if project directory already exists
	if _, err := os.Stat(projectPath); err == nil {
		return fmt.Errorf("directory already exists: %s", projectPath)
	}

	data := templateData{
		Name:            projectName,
		CreatedAt:       time.Now().Format(time.RFC3339),
		StoryTitle:      projectName,
		Genre:           initGenre,
		Premise:         initPremise,
		POV:             initPOV,
		Tense:           "past",
		Language:        initLanguage,
		ProtagonistName: initProtagonist,
		LLMProvider:     initProvider,
		LLMModel:        initModel,
		LLMAPIKey:       initAPIKey,
		LLMTemperature:  0.8,
		LLMMaxTokens:    4096,
	}

	// Interactive wizard
	if initInteractive {
		var err error
		data, err = runWizard(data)
		if err != nil {
			return fmt.Errorf("wizard: %w", err)
		}
	}

	// If no API key provided, check environment
	if data.LLMAPIKey == "" {
		data.LLMAPIKey = resolveAPIKey(data.LLMProvider)
	}

	if err := scaffoldProject(projectPath, data); err != nil {
		return err
	}

	fmt.Printf("✨ Project '%s' created at %s\n", projectName, projectPath)
	fmt.Println()
	fmt.Println("Project structure:")
	fmt.Println("  ├── project.yaml")
	fmt.Println("  ├── world.yaml")
	fmt.Println("  ├── characters/protagonist.yaml")
	fmt.Println("  ├── timeline/")
	fmt.Println("  └── generated/")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Printf("  ff show          # view world state\n")
	fmt.Printf("  ff run           # generate the first chapter\n")

	return nil
}

// scaffoldProject creates a complete project directory from templateData.
// It is shared by `ff init` and by `ff run`'s automatic project bootstrap.
// The target directory may already exist (e.g. ".") as long as it does not
// already contain a project.yaml.
func scaffoldProject(projectPath string, data templateData) error {
	if _, err := os.Stat(filepath.Join(projectPath, "project.yaml")); err == nil {
		return fmt.Errorf("project already exists: %s", projectPath)
	}
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Errorf("create project directory: %w", err)
	}

	// Render and write project.yaml
	tmpl, err := template.New("project.yaml").Parse(projectTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	f, err := os.Create(filepath.Join(projectPath, "project.yaml"))
	if err != nil {
		return fmt.Errorf("create project.yaml: %w", err)
	}
	if err := tmpl.Execute(f, data); err != nil {
		f.Close()
		return fmt.Errorf("render template: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close project.yaml: %w", err)
	}

	// Write world.yaml
	narrativeTime := "Day 0, Prologue"
	if data.Language == "zh" {
		narrativeTime = "第0天，序幕"
	}
	worldContent := fmt.Sprintf(`facts: []
threads: []
chapter_count: 0
event_count: 0
current_narrative_time: "%s"
`, narrativeTime)
	if err := os.WriteFile(filepath.Join(projectPath, "world.yaml"), []byte(worldContent), 0644); err != nil {
		return fmt.Errorf("create world.yaml: %w", err)
	}

	// Create characters directory and protagonist.yaml
	charsDir := filepath.Join(projectPath, "characters")
	if err := os.MkdirAll(charsDir, 0755); err != nil {
		return fmt.Errorf("create characters dir: %w", err)
	}

	protContent := fmt.Sprintf(`name: "%s"
beliefs:
%sgoals:
%sfears:
%svalues:
%smemories: []
abandoned_beliefs: []
`, data.ProtagonistName,
		renderYAMLList(data.InitialBeliefs, 2),
		renderYAMLList(data.InitialGoals, 2),
		renderYAMLList(data.InitialFears, 2),
		renderYAMLList(data.InitialValues, 2))
	if err := os.WriteFile(filepath.Join(charsDir, "protagonist.yaml"), []byte(protContent), 0644); err != nil {
		return fmt.Errorf("create protagonist.yaml: %w", err)
	}

	// Create timeline directory
	if err := os.MkdirAll(filepath.Join(projectPath, "timeline"), 0755); err != nil {
		return fmt.Errorf("create timeline dir: %w", err)
	}

	// Create generated directory
	if err := os.MkdirAll(filepath.Join(projectPath, "generated"), 0755); err != nil {
		return fmt.Errorf("create generated dir: %w", err)
	}

	return nil
}

// renderYAMLList renders a string slice as an indented YAML block list.
func renderYAMLList(items []string, indent int) string {
	if len(items) == 0 {
		return ""
	}
	pad := strings.Repeat(" ", indent)
	var b strings.Builder
	for _, it := range items {
		b.WriteString(pad)
		b.WriteString("- \"")
		b.WriteString(it)
		b.WriteString("\"\n")
	}
	return b.String()
}

func runWizard(data templateData) (templateData, error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("╔═══════════════════════════════════╗")
	fmt.Println("║  Fiction Factory - Setup Wizard  ║")
	fmt.Println("╚═══════════════════════════════════╝")
	fmt.Println()

	// Story title
	fmt.Printf("Story title [%s]: ", data.StoryTitle)
	if scanner.Scan() && scanner.Text() != "" {
		data.StoryTitle = scanner.Text()
	}

	// Genre
	fmt.Printf("Genre (fantasy/sci-fi/mystery/romance/literary) [%s]: ", strOr(data.Genre, "fantasy"))
	if scanner.Scan() && scanner.Text() != "" {
		data.Genre = scanner.Text()
	}
	if data.Genre == "" {
		data.Genre = "fantasy"
	}

	// Premise
	fmt.Print("Premise / one-line summary (optional): ")
	if scanner.Scan() && scanner.Text() != "" {
		data.Premise = scanner.Text()
	}

	// Protagonist name
	fmt.Printf("Protagonist name [%s]: ", data.ProtagonistName)
	if scanner.Scan() && scanner.Text() != "" {
		data.ProtagonistName = scanner.Text()
	}

	// POV
	fmt.Printf("Point of view (first_person/third_person_limited) [%s]: ", data.POV)
	if scanner.Scan() && scanner.Text() != "" {
		data.POV = scanner.Text()
	}

	// Language
	fmt.Printf("Language (zh/en) [%s]: ", data.Language)
	if scanner.Scan() && scanner.Text() != "" {
		data.Language = scanner.Text()
	}

	// LLM provider
	fmt.Printf("LLM provider (deepseek/claude/ollama) [%s]: ", data.LLMProvider)
	if scanner.Scan() && scanner.Text() != "" {
		data.LLMProvider = scanner.Text()
	}

	// LLM model
	defaultModel := defaultModelFor(data.LLMProvider)
	fmt.Printf("LLM model [%s]: ", defaultModel)
	if scanner.Scan() && scanner.Text() != "" {
		data.LLMModel = scanner.Text()
	} else {
		data.LLMModel = defaultModel
	}

	// API key
	fmt.Print("API key (or press enter to use env var): ")
	if scanner.Scan() && scanner.Text() != "" {
		data.LLMAPIKey = scanner.Text()
	}

	fmt.Println()
	return data, nil
}

func resolveAPIKey(provider string) string {
	switch provider {
	case "deepseek":
		if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
			return "${DEEPSEEK_API_KEY}"
		}
	case "claude":
		if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
			return "${ANTHROPIC_API_KEY}"
		}
	case "ollama":
		return "" // ollama doesn't need an API key
	}
	return ""
}

func defaultModelFor(provider string) string {
	switch provider {
	case "deepseek":
		return "deepseek-chat"
	case "claude":
		return "claude-sonnet-4-20250514"
	case "ollama":
		return "llama3.1:8b"
	default:
		return "deepseek-chat"
	}
}

func strOr(val, defaultVal string) string {
	if val == "" {
		return defaultVal
	}
	return val
}
