package check

import (
	"testing"

	"github.com/Paloma966/FF/internal/models"
)

func TestCheckThreads_ForgottenHooks(t *testing.T) {
	world := &models.WorldState{
		Threads: []models.FutureHook{
			{
				ID:          "hook-urgent",
				Description: "An urgent matter",
				Urgency:     "immediate",
				PlantedIn:   "evt-001",
			},
			{
				ID:          "hook-dormant",
				Description: "A dormant thread",
				Urgency:     "dormant",
				PlantedIn:   "evt-001",
			},
		},
	}

	// Create 7 events after the planting
	events := []models.Event{
		{ID: "evt-001", ChapterNum: 1, FutureHooks: []models.FutureHook{
			{ID: "hook-urgent", Description: "An urgent matter", Urgency: "immediate", PlantedIn: "evt-001"},
			{ID: "hook-dormant", Description: "A dormant thread", Urgency: "dormant", PlantedIn: "evt-001"},
		}},
		{ID: "evt-002", ChapterNum: 1},
		{ID: "evt-003", ChapterNum: 2},
		{ID: "evt-004", ChapterNum: 2},
		{ID: "evt-005", ChapterNum: 3},
		{ID: "evt-006", ChapterNum: 3},
		{ID: "evt-007", ChapterNum: 4},
		{ID: "evt-008", ChapterNum: 4},
	}

	issues := CheckThreads(world, events)

	// Should flag the immediate hook as stalled (7 events since planting > maxStalledEvents=5)
	foundImmediate := false
	for _, issue := range issues {
		if issue.Category == "threads" && containsStr(issue.Message, "hook-urgent") {
			foundImmediate = true
			if issue.Severity != SeverityWarning {
				t.Errorf("expected WARNING for stalled immediate hook, got %s", issue.Severity)
			}
		}
	}
	if !foundImmediate {
		t.Error("expected a warning for stalled immediate hook 'hook-urgent'")
	}
}

func TestCheckThreads_DuplicateHooks(t *testing.T) {
	world := &models.WorldState{}

	events := []models.Event{
		{ID: "evt-001", ChapterNum: 1, FutureHooks: []models.FutureHook{
			{ID: "hook-dupe", Description: "A hook", Urgency: "soon", PlantedIn: "evt-001"},
		}},
		{ID: "evt-003", ChapterNum: 2, FutureHooks: []models.FutureHook{
			{ID: "hook-dupe", Description: "Same hook again", Urgency: "soon", PlantedIn: "evt-003"},
		}},
	}

	issues := CheckThreads(world, events)

	foundDupe := false
	for _, issue := range issues {
		if issue.Category == "threads" && containsStr(issue.Message, "重复的钩子 ID") {
			foundDupe = true
			if issue.Severity != SeverityError {
				t.Errorf("expected ERROR for duplicate hook, got %s", issue.Severity)
			}
		}
	}
	if !foundDupe {
		t.Error("expected an error for duplicate hook ID")
	}
}

func TestCheckCharacterDrift_BeliefReversal(t *testing.T) {
	char := &models.Character{
		Name: "Lin En",
		Beliefs: []string{
			"believes the mentor is trustworthy",
		},
		AbandonedBeliefs: []string{
			"believes the mentor is trustworthy",
		},
	}

	issues := CheckCharacterDrift(char, nil)

	foundReversal := false
	for _, issue := range issues {
		if issue.Category == "drift" && containsStr(issue.Message, "同时存在于活跃信念") {
			foundReversal = true
		}
	}
	if !foundReversal {
		t.Error("expected an error for belief in both active and abandoned lists")
	}
}

func TestCheckCharacterDrift_NoIssues(t *testing.T) {
	char := &models.Character{
		Name:    "Lin En",
		Beliefs: []string{"trusts the mentor"},
	}

	issues := CheckCharacterDrift(char, nil)

	if len(issues) > 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

func TestCheckTimeline_ChapterGap(t *testing.T) {
	events := []models.Event{
		{ID: "evt-001", ChapterNum: 1, Participants: []string{"Lin En"}},
		{ID: "evt-002", ChapterNum: 3, Participants: []string{"Lin En"}}, // gap: no chapter 2
	}

	issues := CheckTimeline("Lin En", events)

	foundGap := false
	for _, issue := range issues {
		if issue.Category == "timeline" && containsStr(issue.Message, "章节断层") {
			foundGap = true
		}
	}
	if !foundGap {
		t.Error("expected a warning for chapter gap")
	}
}

func TestRunAll_EmptyProject(t *testing.T) {
	char := &models.Character{Name: "Lin En"}
	world := &models.WorldState{}
	events := []models.Event{}

	report := RunAll(char, world, events)

	if len(report.Issues) > 0 {
		t.Errorf("expected 0 issues for empty project, got %d", len(report.Issues))
	}
}

func TestSimplifyBelief(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"believes the world is fair", "the world is fair"},
		{"suspects the mentor is lying", "the mentor is lying"},
		{"wonders if magic exists", "magic exists"},
		{"fears that he is wrong", "he is wrong"},
		{"no prefix belief", "no prefix belief"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := simplifyBelief(tt.input)
			if got != tt.want {
				t.Errorf("simplifyBelief(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
