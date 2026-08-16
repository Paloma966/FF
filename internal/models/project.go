package models

// Project is the top-level configuration for a fiction project.
type Project struct {
	Name        string            `yaml:"name"`
	CreatedAt   string            `yaml:"created_at"`
	Story       StoryConfig       `yaml:"story"`
	Protagonist ProtagonistConfig `yaml:"protagonist"`
	LLM         LLMConfig         `yaml:"llm"`
}

// StoryConfig holds story-level settings.
type StoryConfig struct {
	Title    string `yaml:"title"`
	Genre    string `yaml:"genre"`
	Premise  string `yaml:"premise"`
	POV      string `yaml:"pov"`      // first_person | third_person_limited
	Tense    string `yaml:"tense"`    // past | present
	Language string `yaml:"language"` // zh | en
}

// ProtagonistConfig holds initial protagonist settings.
type ProtagonistConfig struct {
	Name           string   `yaml:"name"`
	InitialBeliefs []string `yaml:"initial_beliefs"`
	InitialGoals   []string `yaml:"initial_goals"`
	InitialFears   []string `yaml:"initial_fears"`
	InitialValues  []string `yaml:"initial_values"`
}

// LLMConfig holds LLM provider configuration.
type LLMConfig struct {
	Provider    string  `yaml:"provider"` // deepseek | claude | ollama | mock
	Model       string  `yaml:"model"`
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url,omitempty"` // for ollama
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}
