package engine

import (
	"context"
	"testing"

	"github.com/Paloma966/FF/internal/llm"
	"github.com/Paloma966/FF/internal/models"
)

func validDirectorYAML() string {
	return `state_digest: |
  The story has just begun. The protagonist Lin En lives in a border village
  under the shadow of the Empire's war. Tensions are low but the world is
  unsettled. This is the inciting moment.

thread_inventory: []

arc_status: |
  Lin En is at the beginning of the "discovery" arc. He believes his life
  is ordinary and his father's disappearance was a simple accident.
  This belief needs to be challenged.

tension_calibration: |
  Starting from zero. The opening should establish atmosphere and introduce
  the central mystery. Recommend: moderate tension (4-5) to hook the reader
  without overwhelming them.

candidates:
  - title: "The Stranger Arrives"
    description: A wounded soldier collapses at the village gate carrying a message with Lin En's family seal
    threads_used: []
    beliefs_tested:
      - "believes his life is ordinary"
    tension_estimate: 5
  - title: "The Festival Incident"
    description: During the spring festival, Lin En notices someone watching him from the crowd
    threads_used: []
    beliefs_tested:
      - "believes the village is safe"
    tension_estimate: 3
  - title: "The Hidden Letter"
    description: While cleaning the attic, Lin En finds a sealed letter from his father dated the day before he disappeared
    threads_used: []
    beliefs_tested:
      - "believes his father's disappearance was an accident"
    tension_estimate: 6

evaluation:
  scores:
    - title: "The Stranger Arrives"
      thread_progression: 3
      character_arc: 4
      pacing_fit: 5
      consistency: 5
      surprise: 3
    - title: "The Festival Incident"
      thread_progression: 2
      character_arc: 3
      pacing_fit: 5
      consistency: 5
      surprise: 4
    - title: "The Hidden Letter"
      thread_progression: 4
      character_arc: 5
      pacing_fit: 4
      consistency: 5
      surprise: 4
  selected: "The Hidden Letter"
  rationale: "Best balance of character arc advancement and mystery establishment while maintaining consistency"

selected_event:
  title: "The Hidden Letter"
  time: "Day 1, Late Afternoon"
  participants:
    - "Lin En"
  description: "Lin En climbs into the attic to store winter blankets. Behind a loose floorboard, he discovers a sealed envelope bearing his father's handwriting. The letter is dated the day before his father disappeared — but his father supposedly left on a routine trading trip with no warning. The letter's seal is unbroken, meaning it was never sent. When Lin En opens it, the first line reads: 'If you are reading this, I am already gone — and what I told you about the Empire was a lie.'"
  facts_changed:
    - before: "Lin En's father disappeared on a routine trading trip"
      after: "Lin En's father left a hidden letter suggesting his disappearance was planned"
    - before: "Lin En's father was an ordinary merchant"
      after: "Lin En's father had secrets related to the Empire"
  belief_changes:
    - character: "Lin En"
      before: "believes his father's disappearance was an accident"
      after: "begins to suspect his father disappeared deliberately"
  future_hooks:
    - id: "hook-fathers-lie"
      description: "The letter mentions a lie about the Empire — but does not specify what the lie is"
      hints_at: "The Empire may not be what it seems; the father was involved in something dangerous"
      urgency: "soon"
  caused_by: []
  resolves_hooks: []
  director_intent: "Launch the central mystery and give the protagonist a personal stake in uncovering the truth"
  tension_level: 6
  tone: "mysterious"
`
}

func TestDirectorAgent_ParseOutput(t *testing.T) {
	agent := NewDirectorAgent(llm.NewMockLLM())

	yamlText := validDirectorYAML()
	output, err := agent.parseOutput(yamlText)
	if err != nil {
		t.Fatalf("parseOutput failed: %v", err)
	}

	// Check analysis stages
	if output.StateDigest == "" {
		t.Error("state_digest is empty")
	}
	if output.ArcStatus == "" {
		t.Error("arc_status is empty")
	}
	if output.TensionCalibration == "" {
		t.Error("tension_calibration is empty")
	}

	// Check candidates
	if len(output.Candidates) != 3 {
		t.Errorf("expected 3 candidates, got %d", len(output.Candidates))
	}
	if output.Candidates[0].Title != "The Stranger Arrives" {
		t.Errorf("unexpected first candidate: %s", output.Candidates[0].Title)
	}

	// Check evaluation
	if len(output.Evaluation.Scores) != 3 {
		t.Errorf("expected 3 scores, got %d", len(output.Evaluation.Scores))
	}
	if output.Evaluation.Selected != "The Hidden Letter" {
		t.Errorf("expected 'The Hidden Letter' selected, got '%s'", output.Evaluation.Selected)
	}

	// Check selected event
	evt := output.SelectedEvent
	if evt.Title != "The Hidden Letter" {
		t.Errorf("event title: got '%s', want 'The Hidden Letter'", evt.Title)
	}
	if evt.TensionLevel != 6 {
		t.Errorf("tension_level: got %d, want 6", evt.TensionLevel)
	}
	if evt.Tone != "mysterious" {
		t.Errorf("tone: got '%s', want 'mysterious'", evt.Tone)
	}
	if len(evt.FactsChanged) != 2 {
		t.Errorf("expected 2 facts_changed, got %d", len(evt.FactsChanged))
	}
	if len(evt.BeliefChanges) != 1 {
		t.Errorf("expected 1 belief_change, got %d", len(evt.BeliefChanges))
	}
	if len(evt.FutureHooks) != 1 {
		t.Errorf("expected 1 future_hook, got %d", len(evt.FutureHooks))
	}
	if evt.FutureHooks[0].Urgency != "soon" {
		t.Errorf("hook urgency: got '%s', want 'soon'", evt.FutureHooks[0].Urgency)
	}
}

func TestDirectorAgent_ProposeEvent(t *testing.T) {
	mock := llm.NewMockLLM().WithResponses(validDirectorYAML())
	agent := NewDirectorAgent(mock)

	state := DirectorState{
		Facts:            []string{},
		Threads:          nil,
		Protagonist:      &models.Character{Name: "Lin En"},
		RecentEvents:     nil,
		RecentEventCount: 0,
		NextChapterNum:   1,
		CurrentTime:      "Day 0",
	}

	event, output, err := agent.ProposeEvent(context.Background(), state)
	if err != nil {
		t.Fatalf("ProposeEvent failed: %v", err)
	}

	if event.Title != "The Hidden Letter" {
		t.Errorf("event title: got '%s', want 'The Hidden Letter'", event.Title)
	}
	if event.Lifecycle != models.EventProposed {
		t.Errorf("lifecycle: got '%s', want 'proposed'", event.Lifecycle)
	}
	if event.TensionLevel != 6 {
		t.Errorf("tension_level: got %d, want 6", event.TensionLevel)
	}

	if output == nil {
		t.Fatal("output is nil")
	}
	if output.Evaluation.Selected != "The Hidden Letter" {
		t.Errorf("selected: got '%s', want 'The Hidden Letter'", output.Evaluation.Selected)
	}
}

func TestDirectorAgent_ValidateEvent(t *testing.T) {
	agent := NewDirectorAgent(llm.NewMockLLM())

	tests := []struct {
		name    string
		event   RawEvent
		wantErr bool
	}{
		{
			name: "valid event",
			event: RawEvent{
				Title:          "Test Event",
				Time:           "Day 1",
				Description:    "Something happens",
				DirectorIntent: "Test the character",
				TensionLevel:   5,
				Tone:           "mysterious",
			},
			wantErr: false,
		},
		{
			name: "missing title",
			event: RawEvent{
				Time:           "Day 1",
				Description:    "Something happens",
				DirectorIntent: "Test",
				TensionLevel:   5,
				Tone:           "mysterious",
			},
			wantErr: true,
		},
		{
			name: "missing description",
			event: RawEvent{
				Title:          "Test",
				Time:           "Day 1",
				DirectorIntent: "Test",
				TensionLevel:   5,
				Tone:           "mysterious",
			},
			wantErr: true,
		},
		{
			name: "tension out of range (0)",
			event: RawEvent{
				Title:          "Test",
				Time:           "Day 1",
				Description:    "Something",
				DirectorIntent: "Test",
				TensionLevel:   0,
				Tone:           "mysterious",
			},
			wantErr: true,
		},
		{
			name: "tension out of range (11)",
			event: RawEvent{
				Title:          "Test",
				Time:           "Day 1",
				Description:    "Something",
				DirectorIntent: "Test",
				TensionLevel:   11,
				Tone:           "mysterious",
			},
			wantErr: true,
		},
		{
			name: "too many hooks",
			event: RawEvent{
				Title:          "Test",
				Time:           "Day 1",
				Description:    "Something",
				DirectorIntent: "Test",
				TensionLevel:   5,
				Tone:           "mysterious",
				FutureHooks: []RawFutureHook{
					{ID: "h1", Description: "a", HintsAt: "b", Urgency: "soon"},
					{ID: "h2", Description: "c", HintsAt: "d", Urgency: "soon"},
					{ID: "h3", Description: "e", HintsAt: "f", Urgency: "soon"},
					{ID: "h4", Description: "g", HintsAt: "h", Urgency: "soon"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := agent.validateRawEvent(&tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRawEvent() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractYAMLBlock(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "finds state_digest marker",
			input: "Some preamble text\n\nstate_digest: hello\nmore: content",
			want:  "state_digest: hello\nmore: content",
		},
		{
			name:  "falls back to selected_event",
			input: "No state_digest here\nselected_event:\n  title: Test",
			want:  "No state_digest here\nselected_event:\n  title: Test",
		},
		{
			name:  "returns full text as last resort",
			input: "Just some plain text",
			want:  "Just some plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYAMLBlock(tt.input)
			if got != tt.want {
				t.Errorf("extractYAMLBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCandidateScore_TotalScore(t *testing.T) {
	cs := CandidateScore{
		ThreadProgression: 4,
		CharacterArc:      5,
		PacingFit:         3,
		Consistency:       5,
		Surprise:          4,
	}
	if got := cs.TotalScore(); got != 21 {
		t.Errorf("TotalScore() = %d, want 21", got)
	}
}
