package engine

import (
	"context"
	"testing"

	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/models"
)

func validProtagonistYAML() string {
	return `emotional_response: |
  A cold weight settles in my chest. Not surprise exactly — somewhere beneath
  the shock, I think I always knew this moment would come. My hands are steady
  but my throat is tight.

decision: |
  I will not confront him yet. I need more proof. I'll watch. I'll wait.
  I'll find something he cannot explain away.

new_beliefs:
  - "my father was not who he claimed to be"

modified_beliefs:
  - "the world is simple -> the world is layered with secrets"

abandoned_beliefs:
  - "my father's disappearance was an accident"

strengthened_values:
  - "truth-seeking"

challenged_values:
  - "trust in family"

memory_formed: |
  The dust motes floating in the attic light as I held the envelope. The way
  my father's handwriting looked exactly as I remembered it — familiar and
  suddenly alien at the same time.

internal_monologue: |
  He lied to me. My whole life, he lied. But why? What was so terrible that
  he couldn't tell his own son? And why do I feel like I'm standing at the
  edge of something much bigger than a missing father?
`
}

func TestProtagonistAgent_ParseReaction(t *testing.T) {
	agent := NewProtagonistAgent(llm.NewMockLLM())

	reaction, err := agent.parseReaction(validProtagonistYAML())
	if err != nil {
		t.Fatalf("parseReaction failed: %v", err)
	}

	if reaction.EmotionalResponse == "" {
		t.Error("emotional_response is empty")
	}
	if reaction.Decision == "" {
		t.Error("decision is empty")
	}
	if len(reaction.NewBeliefs) != 1 {
		t.Errorf("expected 1 new belief, got %d", len(reaction.NewBeliefs))
	}
	if len(reaction.ModifiedBeliefs) != 1 {
		t.Errorf("expected 1 modified belief, got %d", len(reaction.ModifiedBeliefs))
	}
	if len(reaction.AbandonedBeliefs) != 1 {
		t.Errorf("expected 1 abandoned belief, got %d", len(reaction.AbandonedBeliefs))
	}
	if len(reaction.StrengthenedValues) != 1 {
		t.Errorf("expected 1 strengthened value, got %d", len(reaction.StrengthenedValues))
	}
	if len(reaction.ChallengedValues) != 1 {
		t.Errorf("expected 1 challenged value, got %d", len(reaction.ChallengedValues))
	}
	if reaction.MemoryFormed == "" {
		t.Error("memory_formed is empty")
	}
	if reaction.InternalMonologue == "" {
		t.Error("internal_monologue is empty")
	}
}

func TestProtagonistAgent_ProcessEvent(t *testing.T) {
	mock := llm.NewMockLLM().WithResponses(validProtagonistYAML())
	agent := NewProtagonistAgent(mock)

	state := ProtagonistState{
		Name:    "Lin En",
		Beliefs: []string{"my father's disappearance was an accident", "the world is simple"},
		Goals:   []string{"find the truth about my father"},
		Fears:   []string{"becoming like my father"},
		Values:  []string{"truth-seeking", "trust in family"},
		Event: ProcessedEvent{
			Title:          "The Hidden Letter",
			Time:           "Day 1, Late Afternoon",
			Tone:           "mysterious",
			Description:    "Lin En finds a hidden letter from his father...",
			DirectorIntent: "Launch the central mystery",
		},
	}

	reaction, err := agent.ProcessEvent(context.Background(), state)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}

	if reaction.Decision == "" {
		t.Error("decision is empty")
	}
	if reaction.EmotionalResponse == "" {
		t.Error("emotional_response is empty")
	}
}

func TestApplyReaction(t *testing.T) {
	char := &models.Character{
		Name: "Lin En",
		Beliefs: []string{
			"my father's disappearance was an accident",
			"the world is simple",
		},
		Values: []string{"truth-seeking", "trust in family"},
	}

	reaction := &models.ProtagonistReaction{
		NewBeliefs:         []string{"my father was not who he claimed to be"},
		ModifiedBeliefs:    []string{"the world is simple -> the world is layered with secrets"},
		AbandonedBeliefs:   []string{"my father's disappearance was an accident"},
		StrengthenedValues: []string{"truth-seeking"},
		ChallengedValues:   []string{"trust in family"},
		MemoryFormed:       "The dust in the attic light. The envelope.",
	}

	ApplyReaction(char, "evt-001", reaction, "Day 1, Late Afternoon")

	// Check abandoned belief is removed from active
	for _, b := range char.Beliefs {
		if b == "my father's disappearance was an accident" {
			t.Errorf("abandoned belief still in active beliefs: %s", b)
		}
	}

	// Check abandoned belief is tracked
	found := false
	for _, b := range char.AbandonedBeliefs {
		if b == "my father's disappearance was an accident" {
			found = true
			break
		}
	}
	if !found {
		t.Error("abandoned belief not found in AbandonedBeliefs")
	}

	// Check memory was added
	if len(char.Memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(char.Memories))
	}
	if char.Memories[0].EventID != "evt-001" {
		t.Errorf("memory event ID: got '%s', want 'evt-001'", char.Memories[0].EventID)
	}
	if char.Memories[0].Intensity < 1 || char.Memories[0].Intensity > 10 {
		t.Errorf("intensity out of range: %d", char.Memories[0].Intensity)
	}
}

func TestEstimateIntensity(t *testing.T) {
	tests := []struct {
		name     string
		reaction *models.ProtagonistReaction
		min      int
		max      int
	}{
		{
			name:     "baseline",
			reaction: &models.ProtagonistReaction{},
			min:      5,
			max:      5,
		},
		{
			name: "with abandoned beliefs",
			reaction: &models.ProtagonistReaction{
				AbandonedBeliefs: []string{"old belief"},
			},
			min: 7,
			max: 7,
		},
		{
			name: "with challenged values and abandoned beliefs",
			reaction: &models.ProtagonistReaction{
				AbandonedBeliefs:   []string{"old belief"},
				ChallengedValues:   []string{"loyalty"},
				StrengthenedValues: []string{"truth"},
			},
			min: 8,
			max: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateIntensity(tt.reaction)
			if got < tt.min || got > tt.max {
				t.Errorf("intensity = %d, want between %d and %d", got, tt.min, tt.max)
			}
		})
	}
}

func TestCategorizeMemory(t *testing.T) {
	tests := []struct {
		name     string
		reaction *models.ProtagonistReaction
		want     string
	}{
		{
			name:     "revelation from abandoned beliefs",
			reaction: &models.ProtagonistReaction{AbandonedBeliefs: []string{"old"}},
			want:     "revelation",
		},
		{
			name: "loss from emotional response",
			reaction: &models.ProtagonistReaction{
				EmotionalResponse: "a deep sense of loss washes over me",
			},
			want: "loss",
		},
		{
			name: "triumph",
			reaction: &models.ProtagonistReaction{
				EmotionalResponse: "I feel a surge of triumph",
			},
			want: "triumph",
		},
		{
			name: "trauma from fear",
			reaction: &models.ProtagonistReaction{
				EmotionalResponse: "terror grips my heart",
			},
			want: "trauma",
		},
		{
			name:     "default connection",
			reaction: &models.ProtagonistReaction{},
			want:     "connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeMemory(tt.reaction)
			if got != tt.want {
				t.Errorf("categorizeMemory() = %s, want %s", got, tt.want)
			}
		})
	}
}
