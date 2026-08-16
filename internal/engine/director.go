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

// DirectorAgent is responsible for story direction, event design, and pacing.
type DirectorAgent struct {
	llm llm.LLMClient
}

// NewDirectorAgent creates a new DirectorAgent with the given LLM client.
func NewDirectorAgent(llmClient llm.LLMClient) *DirectorAgent {
	return &DirectorAgent{llm: llmClient}
}

// DirectorState is the input state for the Director's analysis.
type DirectorState struct {
	Facts            []string
	Threads          []models.FutureHook
	Protagonist      *models.Character
	RecentMemories   []models.Memory
	RecentEvents     []models.Event
	RecentEventCount int
	NextChapterNum   int
	CurrentTime      string
}

// DirectorOutput is the structured YAML the Director returns.
type DirectorOutput struct {
	StateDigest        string           `yaml:"state_digest"`
	ThreadInventory    []ThreadVerdict  `yaml:"thread_inventory"`
	ArcStatus          string           `yaml:"arc_status"`
	TensionCalibration string           `yaml:"tension_calibration"`
	Candidates         []EventCandidate `yaml:"candidates"`
	Evaluation         Evaluation       `yaml:"evaluation"`
	SelectedEvent      RawEvent         `yaml:"selected_event"`
}

// ThreadVerdict is the Director's judgment on each unresolved hook.
type ThreadVerdict struct {
	HookID    string `yaml:"hook_id"`
	Judgment  string `yaml:"judgment"` // "pay_off_now" | "develop" | "simmer"
	Rationale string `yaml:"rationale"`
}

// EventCandidate is a brief event idea proposed by the Director.
type EventCandidate struct {
	Title           string   `yaml:"title"`
	Description     string   `yaml:"description"`
	ThreadsUsed     []string `yaml:"threads_used"`
	BeliefsTested   []string `yaml:"beliefs_tested"`
	TensionEstimate int      `yaml:"tension_estimate"`
}

// Evaluation contains the Director's scoring and selection.
type Evaluation struct {
	Scores    []CandidateScore `yaml:"scores"`
	Selected  string           `yaml:"selected"`
	Rationale string           `yaml:"rationale"`
}

// CandidateScore is the 5-dimension score for a candidate event.
type CandidateScore struct {
	Title             string `yaml:"title"`
	ThreadProgression int    `yaml:"thread_progression"`
	CharacterArc      int    `yaml:"character_arc"`
	PacingFit         int    `yaml:"pacing_fit"`
	Consistency       int    `yaml:"consistency"`
	Surprise          int    `yaml:"surprise"`
}

// TotalScore returns the summed score for a candidate.
func (cs CandidateScore) TotalScore() int {
	return cs.ThreadProgression + cs.CharacterArc + cs.PacingFit + cs.Consistency + cs.Surprise
}

// RawEvent is the event as parsed from the Director's YAML output.
// It mirrors models.Event but all fields are strings/simple types from YAML parsing.
type RawEvent struct {
	Title          string                `yaml:"title"`
	Time           string                `yaml:"time"`
	Participants   []string              `yaml:"participants"`
	Description    string                `yaml:"description"`
	FactsChanged   []models.FactChange   `yaml:"facts_changed"`
	BeliefChanges  []models.BeliefChange `yaml:"belief_changes"`
	FutureHooks    []RawFutureHook       `yaml:"future_hooks"`
	CausedBy       []models.EventRef     `yaml:"caused_by"`
	ResolvesHooks  []string              `yaml:"resolves_hooks"`
	DirectorIntent string                `yaml:"director_intent"`
	TensionLevel   int                   `yaml:"tension_level"`
	Tone           string                `yaml:"tone"`
}

// RawFutureHook is a FutureHook as parsed from YAML (without derived fields).
type RawFutureHook struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	HintsAt     string `yaml:"hints_at"`
	Urgency     string `yaml:"urgency"`
}

// ProposeEvent runs the full seven-stage Director analysis and returns
// the selected event and the full DirectorOutput for logging.
func (d *DirectorAgent) ProposeEvent(ctx context.Context, state DirectorState) (*models.Event, *DirectorOutput, error) {
	// 1. Render the task prompt from template
	var buf bytes.Buffer
	tmpl, err := template.New("director_task").Parse(prompts.DirectorTaskTemplate)
	if err != nil {
		return nil, nil, fmt.Errorf("parse task template: %w", err)
	}
	if err := tmpl.Execute(&buf, state); err != nil {
		return nil, nil, fmt.Errorf("render task template: %w", err)
	}

	// 2. Call LLM
	resp, err := d.llm.Complete(ctx, llm.CompletionRequest{
		SystemPrompt: prompts.DirectorSystem,
		UserPrompt:   buf.String(),
		Temperature:  0.8,
		MaxTokens:    4096,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("llm call: %w", err)
	}

	// 3. Parse the response as DirectorOutput YAML
	output, err := d.parseOutput(resp.Text)
	if err != nil {
		return nil, nil, fmt.Errorf("parse director output: %w", err)
	}

	// 4. Validate the SelectedEvent
	if err := d.validateRawEvent(&output.SelectedEvent); err != nil {
		return nil, output, fmt.Errorf("validate event: %w", err)
	}

	// 5. Convert RawEvent to models.Event
	event := d.convertToEvent(&output.SelectedEvent)
	event.Lifecycle = models.EventProposed

	return event, output, nil
}

// parseOutput extracts the YAML from the LLM response and unmarshals it.
func (d *DirectorAgent) parseOutput(text string) (*DirectorOutput, error) {
	// Extract YAML block: find the first "state_digest:" and parse from there
	yamlText := extractYAMLBlock(text)

	var output DirectorOutput
	if err := yaml.Unmarshal([]byte(yamlText), &output); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w\nraw text:\n%s", err, text)
	}

	// Sanity checks
	if output.SelectedEvent.Title == "" {
		return nil, fmt.Errorf("selected_event.title is empty")
	}

	return &output, nil
}

// extractYAMLBlock finds the YAML content in the LLM response.
func extractYAMLBlock(text string) string {
	// Look for the YAML document start marker "state_digest:"
	marker := "state_digest:"
	idx := strings.Index(text, marker)
	if idx >= 0 {
		return text[idx:]
	}

	// Fallback: try to find any YAML-like structure
	// Look for "selected_event:" as a fallback marker
	marker = "selected_event:"
	idx = strings.Index(text, marker)
	if idx >= 0 {
		// Include everything — the YAML parser will get what it can
		return text
	}

	// Last resort: return the whole text
	return text
}

// validateRawEvent checks that the raw event has all required fields.
func (d *DirectorAgent) validateRawEvent(evt *RawEvent) error {
	var errs []string

	if evt.Title == "" {
		errs = append(errs, "title is empty")
	}
	if evt.Description == "" {
		errs = append(errs, "description is empty")
	}
	if evt.Time == "" {
		errs = append(errs, "time is empty")
	}
	if evt.DirectorIntent == "" {
		errs = append(errs, "director_intent is empty")
	}
	if evt.TensionLevel < 1 || evt.TensionLevel > 10 {
		errs = append(errs, fmt.Sprintf("tension_level %d out of range (1-10)", evt.TensionLevel))
	}
	if evt.Tone == "" {
		errs = append(errs, "tone is empty")
	}
	if len(evt.FutureHooks) > 3 {
		errs = append(errs, fmt.Sprintf("too many future_hooks: %d (max 3)", len(evt.FutureHooks)))
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// convertToEvent converts a RawEvent (from YAML parsing) to a models.Event.
func (d *DirectorAgent) convertToEvent(raw *RawEvent) *models.Event {
	evt := &models.Event{
		Title:          raw.Title,
		Time:           raw.Time,
		Participants:   raw.Participants,
		Description:    raw.Description,
		FactsChanged:   raw.FactsChanged,
		BeliefChanges:  raw.BeliefChanges,
		CausedBy:       raw.CausedBy,
		ResolvesHooks:  raw.ResolvesHooks,
		DirectorIntent: raw.DirectorIntent,
		TensionLevel:   raw.TensionLevel,
		Tone:           raw.Tone,
	}

	// Convert FutureHooks
	for _, hook := range raw.FutureHooks {
		evt.FutureHooks = append(evt.FutureHooks, models.FutureHook{
			ID:          hook.ID,
			Description: hook.Description,
			HintsAt:     hook.HintsAt,
			Urgency:     hook.Urgency,
		})
	}

	return evt
}

// PrintAnalysis prints the Director's analysis stages to the console for user review.
func (d *DirectorOutput) PrintAnalysis() {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("📋 DIRECTOR ANALYSIS")
	fmt.Println(strings.Repeat("─", 60))

	fmt.Println("\n📍 STAGE 1: STATE DIGEST")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println(wrapText(d.StateDigest, 70))

	fmt.Println("\n🧵 STAGE 2: THREAD INVENTORY")
	fmt.Println(strings.Repeat("─", 40))
	for _, tv := range d.ThreadInventory {
		icon := threadIcon(tv.Judgment)
		fmt.Printf("  %s [%s] %s\n", icon, tv.HookID, tv.Rationale)
	}
	if len(d.ThreadInventory) == 0 {
		fmt.Println("  (no unresolved threads)")
	}

	fmt.Println("\n🎭 STAGE 3: CHARACTER ARC STATUS")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println(wrapText(d.ArcStatus, 70))

	fmt.Println("\n📈 STAGE 4: TENSION CALIBRATION")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println(wrapText(d.TensionCalibration, 70))

	fmt.Println("\n💡 STAGE 5: EVENT CANDIDATES")
	fmt.Println(strings.Repeat("─", 40))
	for i, c := range d.Candidates {
		fmt.Printf("  %d. %s (tension: %d/10)\n", i+1, c.Title, c.TensionEstimate)
		fmt.Printf("     %s\n", wrapText(c.Description, 64))
	}

	fmt.Println("\n⚖️  STAGE 6: EVALUATION")
	fmt.Println(strings.Repeat("─", 40))
	for _, s := range d.Evaluation.Scores {
		fmt.Printf("  %s: %d/25\n", s.Title, s.TotalScore())
	}
	fmt.Printf("\n  Selected: %s\n", d.Evaluation.Selected)
	fmt.Printf("  Rationale: %s\n", wrapText(d.Evaluation.Rationale, 64))

	fmt.Println("\n🎬 STAGE 7: SELECTED EVENT")
	fmt.Println(strings.Repeat("─", 40))
	evt := d.SelectedEvent
	fmt.Printf("  Title: %s\n", evt.Title)
	fmt.Printf("  Time: %s\n", evt.Time)
	fmt.Printf("  Tone: %s | Tension: %d/10\n", evt.Tone, evt.TensionLevel)
	fmt.Printf("  Director Intent: %s\n", evt.DirectorIntent)
	fmt.Printf("  Description: %s\n", wrapText(evt.Description, 64))
	if len(evt.FactsChanged) > 0 {
		fmt.Println("  Facts Changed:")
		for _, fc := range evt.FactsChanged {
			fmt.Printf("    Before: %s\n", fc.Before)
			fmt.Printf("    After:  %s\n", fc.After)
		}
	}
	if len(evt.BeliefChanges) > 0 {
		fmt.Println("  Belief Changes:")
		for _, bc := range evt.BeliefChanges {
			fmt.Printf("    %s: \"%s\" → \"%s\"\n", bc.Character, bc.Before, bc.After)
		}
	}
	if len(evt.FutureHooks) > 0 {
		fmt.Println("  Future Hooks Planted:")
		for _, h := range evt.FutureHooks {
			fmt.Printf("    [%s] %s (urgency: %s)\n", h.ID, h.Description, h.Urgency)
		}
	}
	if len(evt.ResolvesHooks) > 0 {
		fmt.Println("  Hooks Resolved:")
		for _, h := range evt.ResolvesHooks {
			fmt.Printf("    ✓ %s\n", h)
		}
	}
	fmt.Println(strings.Repeat("─", 60))
}

func threadIcon(judgment string) string {
	switch judgment {
	case "pay_off_now":
		return "🔔"
	case "develop":
		return "📌"
	case "simmer":
		return "💤"
	default:
		return "❓"
	}
}

// wrapText wraps long text to a maximum width.
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}
	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0
	for i, word := range words {
		if lineLen+len(word)+1 > width && lineLen > 0 {
			result.WriteString("\n     ")
			lineLen = 0
		}
		if i > 0 && lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}
	return result.String()
}
