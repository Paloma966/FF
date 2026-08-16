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

// VerificationVerdict is the structured verdict of one verification pass.
// approved=false requires at least one concrete issue.
type VerificationVerdict struct {
	Approved    bool     `yaml:"approved"`
	Issues      []string `yaml:"issues"`
	Suggestions string   `yaml:"suggestions"`
}

// VerifyTrace records the outcome of a verify→revise loop for logging.
type VerifyTrace struct {
	Revisions int      // revision rounds actually performed
	Forced    bool     // true when accepted after exhausting all rounds
	Issues    []string // issues from the last failed verdict (empty when approved)
}

// VerifyEventInput is everything the Protagonist needs to verify a Director event.
type VerifyEventInput struct {
	Character    *models.Character
	Facts        []string
	Threads      []models.FutureHook
	RecentEvents []models.Event
	Event        *models.Event
}

// VerifyReactionInput is everything the Director needs to verify a reaction.
type VerifyReactionInput struct {
	Event     *models.Event
	Character *models.Character
	Facts     []string
	Reaction  *models.ProtagonistReaction
}

// VerifyEvent has the Protagonist Agent verify the Director's proposed event
// from the character's psychological perspective.
func (p *ProtagonistAgent) VerifyEvent(ctx context.Context, in VerifyEventInput) (VerificationVerdict, error) {
	eventYAML, err := marshalYAML(in.Event)
	if err != nil {
		return VerificationVerdict{}, fmt.Errorf("marshal event: %w", err)
	}

	type taskData struct {
		Character    *models.Character
		Facts        []string
		Threads      []models.FutureHook
		RecentEvents []models.Event
		EventYAML    string
	}
	var buf bytes.Buffer
	tmpl, err := template.New("verify_event_task").Parse(prompts.VerifyEventTaskTemplate)
	if err != nil {
		return VerificationVerdict{}, fmt.Errorf("parse verify template: %w", err)
	}
	if err := tmpl.Execute(&buf, taskData{
		Character:    in.Character,
		Facts:        in.Facts,
		Threads:      in.Threads,
		RecentEvents: in.RecentEvents,
		EventYAML:    eventYAML,
	}); err != nil {
		return VerificationVerdict{}, fmt.Errorf("render verify template: %w", err)
	}

	resp, err := p.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.VerifyEventSystem,
		UserPrompt:   buf.String(),
		Temperature:  0.3,
		MaxTokens:    1024,
	})
	if err != nil {
		return VerificationVerdict{}, fmt.Errorf("verify llm call: %w", err)
	}
	return parseVerdict(resp.Text)
}

// VerifyReaction has the Director Agent verify the Protagonist's reaction
// from the narrative logic perspective.
func (d *DirectorAgent) VerifyReaction(ctx context.Context, in VerifyReactionInput) (VerificationVerdict, error) {
	reactionYAML, err := marshalYAML(in.Reaction)
	if err != nil {
		return VerificationVerdict{}, fmt.Errorf("marshal reaction: %w", err)
	}

	type taskData struct {
		Event        *models.Event
		Character    *models.Character
		Facts        []string
		ReactionYAML string
	}
	var buf bytes.Buffer
	tmpl, err := template.New("verify_reaction_task").Parse(prompts.VerifyReactionTaskTemplate)
	if err != nil {
		return VerificationVerdict{}, fmt.Errorf("parse verify template: %w", err)
	}
	if err := tmpl.Execute(&buf, taskData{
		Event:        in.Event,
		Character:    in.Character,
		Facts:        in.Facts,
		ReactionYAML: reactionYAML,
	}); err != nil {
		return VerificationVerdict{}, fmt.Errorf("render verify template: %w", err)
	}

	resp, err := d.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.VerifyReactionSystem,
		UserPrompt:   buf.String(),
		Temperature:  0.3,
		MaxTokens:    1024,
	})
	if err != nil {
		return VerificationVerdict{}, fmt.Errorf("verify llm call: %w", err)
	}
	return parseVerdict(resp.Text)
}

// ReviseEvent has the Director revise a rejected event using the issues.
func (d *DirectorAgent) ReviseEvent(ctx context.Context, event *models.Event, verdict VerificationVerdict) (*models.Event, error) {
	eventYAML, err := marshalYAML(event)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}

	type taskData struct {
		Issues      []string
		Suggestions string
		EventYAML   string
	}
	var buf bytes.Buffer
	tmpl, err := template.New("revise_event_task").Parse(prompts.ReviseEventTaskTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse revise template: %w", err)
	}
	if err := tmpl.Execute(&buf, taskData{
		Issues:      verdict.Issues,
		Suggestions: verdict.Suggestions,
		EventYAML:   eventYAML,
	}); err != nil {
		return nil, fmt.Errorf("render revise template: %w", err)
	}

	resp, err := d.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.DirectorSystem,
		UserPrompt:   buf.String(),
		Temperature:  0.7,
		MaxTokens:    4096,
	})
	if err != nil {
		return nil, fmt.Errorf("revise llm call: %w", err)
	}

	// The revised output is a YAML document rooted at selected_event.
	var wrapper struct {
		SelectedEvent RawEvent `yaml:"selected_event"`
	}
	yamlText := extractYAMLBlock(resp.Text)
	idx := strings.Index(yamlText, "selected_event:")
	if idx >= 0 {
		yamlText = yamlText[idx:]
	}
	if err := yaml.Unmarshal([]byte(yamlText), &wrapper); err != nil {
		return nil, fmt.Errorf("parse revised event: %w\nraw text:\n%s", err, resp.Text)
	}
	if err := d.validateRawEvent(&wrapper.SelectedEvent); err != nil {
		return nil, fmt.Errorf("validate revised event: %w", err)
	}
	revised := d.convertToEvent(&wrapper.SelectedEvent)
	revised.Lifecycle = models.EventProposed
	return revised, nil
}

// ReviseReaction has the Protagonist revise a rejected reaction using the issues.
func (p *ProtagonistAgent) ReviseReaction(ctx context.Context, event *models.Event, reaction *models.ProtagonistReaction, verdict VerificationVerdict) (*models.ProtagonistReaction, error) {
	reactionYAML, err := marshalYAML(reaction)
	if err != nil {
		return nil, fmt.Errorf("marshal reaction: %w", err)
	}
	summary := fmt.Sprintf("标题: %s\n时间: %s\n经过: %s", event.Title, event.Time, event.Description)

	type taskData struct {
		Issues       []string
		Suggestions  string
		EventSummary string
		ReactionYAML string
	}
	var buf bytes.Buffer
	tmpl, err := template.New("revise_reaction_task").Parse(prompts.ReviseReactionTaskTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse revise template: %w", err)
	}
	if err := tmpl.Execute(&buf, taskData{
		Issues:       verdict.Issues,
		Suggestions:  verdict.Suggestions,
		EventSummary: summary,
		ReactionYAML: reactionYAML,
	}); err != nil {
		return nil, fmt.Errorf("render revise template: %w", err)
	}

	resp, err := p.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.ProtagonistSystem,
		UserPrompt:   buf.String(),
		Temperature:  0.7,
		MaxTokens:    2048,
	})
	if err != nil {
		return nil, fmt.Errorf("revise llm call: %w", err)
	}
	return p.parseReaction(resp.Text)
}

// parseVerdict extracts and unmarshals a VerificationVerdict from LLM text.
func parseVerdict(text string) (VerificationVerdict, error) {
	yamlText := extractYAMLBlock(text)
	idx := strings.Index(yamlText, "approved:")
	if idx < 0 {
		return VerificationVerdict{}, fmt.Errorf("verdict missing 'approved' field\nraw text:\n%s", text)
	}
	var v VerificationVerdict
	if err := yaml.Unmarshal([]byte(yamlText[idx:]), &v); err != nil {
		return VerificationVerdict{}, fmt.Errorf("verdict yaml unmarshal: %w\nraw text:\n%s", err, text)
	}
	if !v.Approved && len(v.Issues) == 0 {
		v.Issues = []string{"(verifier 未给出具体问题)"}
	}
	return v, nil
}

// marshalYAML serializes any YAML-tagged struct for prompt embedding.
func marshalYAML(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
