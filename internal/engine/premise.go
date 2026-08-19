package engine

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/prompts"
	"gopkg.in/yaml.v3"
)

// Premise is the structured output of the PremiseGenerator: everything needed
// to bootstrap a brand-new novel project from a trending topic.
type Premise struct {
	Title           string   `yaml:"title"`
	Genre           string   `yaml:"genre"`
	Premise         string   `yaml:"premise"`
	ProtagonistName string   `yaml:"protagonist_name"`
	InitialBeliefs  []string `yaml:"initial_beliefs"`
	InitialGoals    []string `yaml:"initial_goals"`
	InitialFears    []string `yaml:"initial_fears"`
	InitialValues   []string `yaml:"initial_values"`
}

// PremiseGenerator turns a real-world trending topic into an original
// novel premise plus a fully-formed protagonist.
type PremiseGenerator struct {
	llm llm.LLMClient
}

// NewPremiseGenerator creates a PremiseGenerator.
func NewPremiseGenerator(llmClient llm.LLMClient) *PremiseGenerator {
	return &PremiseGenerator{llm: llmClient}
}

// PremiseInput is the trending topic material fed to the generator.
type PremiseInput struct {
	TopicTitle     string
	HeatLabel      string
	Summary        string
	Source         string
	URL            string
	Genre          string // optional override; empty lets the model choose
	Language       string // "zh" | "en"; controls the output-language directive
	OutputLanguage string // rendered directive; set by Generate
}

// Generate produces a novel premise from the topic.
func (p *PremiseGenerator) Generate(ctx context.Context, in PremiseInput) (*Premise, error) {
	in.OutputLanguage = outputLanguageDirective(in.Language)
	var buf bytes.Buffer
	tmpl, err := template.New("premise_task").Parse(prompts.PremiseTaskTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse premise template: %w", err)
	}
	if err := tmpl.Execute(&buf, in); err != nil {
		return nil, fmt.Errorf("render premise template: %w", err)
	}

	resp, err := p.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.PremiseSystem,
		UserPrompt:   buf.String(),
		Temperature:  0.9,
		MaxTokens:    2048,
	})
	if err != nil {
		return nil, fmt.Errorf("premise llm call: %w", err)
	}

	premise, err := parsePremise(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parse premise: %w", err)
	}
	return premise, nil
}

// parsePremise extracts and unmarshals the premise YAML from LLM text.
func parsePremise(text string) (*Premise, error) {
	yamlText := extractYAMLBlock(text)
	idx := strings.Index(yamlText, "title:")
	if idx >= 0 {
		yamlText = yamlText[idx:]
	}

	var premise Premise
	if err := yaml.Unmarshal([]byte(yamlText), &premise); err != nil {
		return nil, fmt.Errorf("premise yaml unmarshal: %w\nraw text:\n%s", err, text)
	}

	if strings.TrimSpace(premise.Title) == "" {
		return nil, fmt.Errorf("premise title is empty")
	}
	if strings.TrimSpace(premise.Premise) == "" {
		return nil, fmt.Errorf("premise is empty")
	}
	if strings.TrimSpace(premise.ProtagonistName) == "" {
		return nil, fmt.Errorf("protagonist_name is empty")
	}
	if premise.Genre == "" {
		premise.Genre = "都市小说"
	}

	return &premise, nil
}
