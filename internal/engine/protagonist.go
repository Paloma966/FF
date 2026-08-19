package engine

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/models"
	"github.com/chun/fiction_factory/internal/prompts"
	"gopkg.in/yaml.v3"
)

// ProtagonistAgent embodies the protagonist, processing events psychologically.
type ProtagonistAgent struct {
	llm         llm.LLMClient
	temperature float64
	maxTokens   int
	language    string
}

// NewProtagonistAgent creates a new ProtagonistAgent.
func NewProtagonistAgent(llmClient llm.LLMClient) *ProtagonistAgent {
	return &ProtagonistAgent{llm: llmClient, temperature: 0.9, maxTokens: 2048, language: "zh"}
}

// setLLMSettings overrides the default temperature/maxTokens when non-zero.
func (p *ProtagonistAgent) setLLMSettings(temperature float64, maxTokens int) {
	if temperature > 0 {
		p.temperature = temperature
	}
	if maxTokens > 0 {
		p.maxTokens = maxTokens
	}
}

// setLanguage overrides the output language ("zh" or "en").
func (p *ProtagonistAgent) setLanguage(lang string) {
	if lang != "" {
		p.language = lang
	}
}

// ProtagonistState is the input state for the protagonist's response.
type ProtagonistState struct {
	Name             string
	Beliefs          []string
	Goals            []string
	Fears            []string
	Values           []string
	RecentMemories   []models.Memory
	Event            ProcessedEvent
	OutputLanguage   string
}

// ProcessedEvent is a simplified view of an event for the protagonist prompt.
type ProcessedEvent struct {
	Title          string
	Time           string
	Tone           string
	Description    string
	FactsChanged   []models.FactChange
	Participants   []string
	DirectorIntent string
}

// ProcessEvent runs the protagonist's psychological processing of an event.
func (p *ProtagonistAgent) ProcessEvent(ctx context.Context, state ProtagonistState) (*models.ProtagonistReaction, error) {
	state.OutputLanguage = outputLanguageDirective(p.language)
	var buf bytes.Buffer
	tmpl, err := template.New("protagonist_task").Parse(prompts.ProtagonistTaskTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse task template: %w", err)
	}
	if err := tmpl.Execute(&buf, state); err != nil {
		return nil, fmt.Errorf("render task template: %w", err)
	}

	resp, err := p.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.ProtagonistSystem,
		UserPrompt:   buf.String(),
		Temperature:  p.temperature,
		MaxTokens:    p.maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	reaction, err := p.parseReaction(resp.Text)
	if err != nil {
		return nil, fmt.Errorf("parse protagonist reaction: %w", err)
	}

	return reaction, nil
}

// parseReaction extracts the YAML from the LLM response and unmarshals it.
func (p *ProtagonistAgent) parseReaction(text string) (*models.ProtagonistReaction, error) {
	yamlText := extractYAMLBlock(text)

	// Try to find the start of the reaction YAML
	marker := "emotional_response:"
	idx := strings.Index(yamlText, marker)
	if idx >= 0 {
		yamlText = yamlText[idx:]
	}

	var reaction models.ProtagonistReaction
	if err := yaml.Unmarshal([]byte(yamlText), &reaction); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w\nraw text:\n%s", err, text)
	}

	// Validate
	if reaction.EmotionalResponse == "" {
		return nil, fmt.Errorf("emotional_response is empty")
	}
	if reaction.Decision == "" {
		return nil, fmt.Errorf("decision is empty")
	}

	return &reaction, nil
}

// ApplyReaction updates a character's state based on a ProtagonistReaction.
func ApplyReaction(char *models.Character, eventID string, reaction *models.ProtagonistReaction, narrativeTime string) {
	// Apply belief changes
	for _, b := range reaction.NewBeliefs {
		char.Beliefs = append(char.Beliefs, b)
	}
	for _, b := range reaction.ModifiedBeliefs {
		char.Beliefs = append(char.Beliefs, b)
	}
	for _, b := range reaction.AbandonedBeliefs {
		char.AbandonedBeliefs = append(char.AbandonedBeliefs, b)
		// Remove from active beliefs
		char.Beliefs = removeString(char.Beliefs, b)
	}

	// Add the memory
	if reaction.MemoryFormed != "" {
		memory := models.Memory{
			ID:        fmt.Sprintf("mem-%s", eventID),
			EventID:   eventID,
			Content:   reaction.MemoryFormed,
			Intensity: estimateIntensity(reaction),
			Category:  categorizeMemory(reaction),
			Timestamp: narrativeTime,
		}
		char.Memories = append(char.Memories, memory)
	}
}

// removeString removes a string from a slice.
func removeString(slice []string, target string) []string {
	var result []string
	for _, s := range slice {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}

// estimateIntensity estimates memory intensity from the reaction.
func estimateIntensity(r *models.ProtagonistReaction) int {
	intensity := 5 // default

	// Higher intensity if beliefs were abandoned (significant psychological event)
	if len(r.ChallengedValues) > 0 {
		intensity += 2
	}
	if len(r.AbandonedBeliefs) > 0 {
		intensity += 2
	}
	if len(r.StrengthenedValues) > 0 || len(r.ChallengedValues) > 0 {
		intensity += 1
	}

	if intensity > 10 {
		intensity = 10
	}
	return intensity
}

// categorizeMemory determines the memory category from the reaction.
func categorizeMemory(r *models.ProtagonistReaction) string {
	if len(r.AbandonedBeliefs) > 0 {
		return "revelation"
	}
	if len(r.StrengthenedValues) > 0 && len(r.ChallengedValues) > 0 {
		return "connection"
	}
	if strings.Contains(strings.ToLower(r.EmotionalResponse), "loss") ||
		strings.Contains(strings.ToLower(r.EmotionalResponse), "grief") ||
		strings.Contains(strings.ToLower(r.EmotionalResponse), "empty") {
		return "loss"
	}
	if strings.Contains(strings.ToLower(r.EmotionalResponse), "triumph") ||
		strings.Contains(strings.ToLower(r.EmotionalResponse), "joy") ||
		strings.Contains(strings.ToLower(r.EmotionalResponse), "proud") {
		return "triumph"
	}
	if strings.Contains(strings.ToLower(r.EmotionalResponse), "fear") ||
		strings.Contains(strings.ToLower(r.EmotionalResponse), "terror") ||
		strings.Contains(strings.ToLower(r.EmotionalResponse), "dread") {
		return "trauma"
	}
	return "connection"
}
