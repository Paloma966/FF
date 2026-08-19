package engine

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/models"
	"github.com/chun/fiction_factory/internal/prompts"
)

// ChapterGenerator produces prose from Director + Protagonist outputs.
type ChapterGenerator struct {
	llm         llm.LLMClient
	temperature float64
	maxTokens   int
	language    string
}

// NewChapterGenerator creates a new ChapterGenerator.
func NewChapterGenerator(llmClient llm.LLMClient) *ChapterGenerator {
	return &ChapterGenerator{llm: llmClient, temperature: 0.8, maxTokens: 4096, language: "zh"}
}

// setLLMSettings overrides the default temperature/maxTokens when non-zero.
func (cg *ChapterGenerator) setLLMSettings(temperature float64, maxTokens int) {
	if temperature > 0 {
		cg.temperature = temperature
	}
	if maxTokens > 0 {
		cg.maxTokens = maxTokens
	}
}

// setLanguage overrides the output language ("zh" or "en").
func (cg *ChapterGenerator) setLanguage(lang string) {
	if lang != "" {
		cg.language = lang
	}
}

// ChapterInput is the assembled input for chapter generation.
type ChapterInput struct {
	ChapterNum        int
	Time              string
	Tone              string
	POV               string
	Tense             string
	PreviousSummary   string
	Description       string
	DirectorIntent    string
	FactsChanged      []models.FactChange
	ResolvedHooks     []string
	EmotionalResponse string
	InternalMonologue string
	Decision          string
	NewBeliefs        []string
	ModifiedBeliefs   []string
	OutputLanguage    string
}

// Generate writes chapter prose from the assembled state.
func (cg *ChapterGenerator) Generate(ctx context.Context, input ChapterInput) (string, error) {
	input.OutputLanguage = outputLanguageDirective(cg.language)
	var buf bytes.Buffer
	tmpl, err := template.New("chapter_task").Parse(prompts.ChapterTaskTemplate)
	if err != nil {
		return "", fmt.Errorf("parse chapter template: %w", err)
	}
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("render chapter template: %w", err)
	}

	resp, err := cg.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.ChapterSystem,
		UserPrompt:   buf.String(),
		Temperature:  cg.temperature,
		MaxTokens:    cg.maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("llm call: %w", err)
	}

	// Chapter output is prose — no YAML parsing needed
	text := resp.Text
	if text == "" {
		return "", fmt.Errorf("empty chapter text")
	}

	return text, nil
}

// ChapterInputFromEvent builds a ChapterInput from event and reaction data.
func ChapterInputFromEvent(
	event *models.Event,
	reaction *models.ProtagonistReaction,
	pov string,
	tense string,
	prevSummary string,
) ChapterInput {
	input := ChapterInput{
		ChapterNum:      event.ChapterNum,
		Time:            event.Time,
		Tone:            event.Tone,
		POV:             pov,
		Tense:           tense,
		PreviousSummary: prevSummary,
		Description:     event.Description,
		DirectorIntent:  event.DirectorIntent,
		FactsChanged:    event.FactsChanged,
		ResolvedHooks:   event.ResolvesHooks,
	}

	if reaction != nil {
		input.EmotionalResponse = reaction.EmotionalResponse
		input.InternalMonologue = reaction.InternalMonologue
		input.Decision = reaction.Decision
		input.NewBeliefs = reaction.NewBeliefs
		input.ModifiedBeliefs = reaction.ModifiedBeliefs
	}

	return input
}
