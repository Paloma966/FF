package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Paloma966/FF/internal/storage"
)

func TestScaffoldProjectRoundtrip(t *testing.T) {
	dir := t.TempDir()
	data := templateData{
		Name:            "hot-novel",
		CreatedAt:       "2026-01-01T00:00:00Z",
		StoryTitle:      "城郊来电",
		Genre:           "都市悬疑",
		Premise:         "一个外卖骑手的悬疑故事",
		POV:             "third_person_limited",
		Tense:           "past",
		Language:        "zh",
		ProtagonistName: "陈野",
		InitialBeliefs:  []string{"妻子的失踪是意外"},
		InitialGoals:    []string{"查明真相"},
		InitialFears:    []string{"真相会毁掉自己"},
		InitialValues:   []string{"诚实", "自保"},
		LLMProvider:     "deepseek",
		LLMModel:        "deepseek-chat",
		LLMAPIKey:       "${DEEPSEEK_API_KEY}",
		LLMTemperature:  0.8,
		LLMMaxTokens:    4096,
	}
	if err := scaffoldProject(dir, data); err != nil {
		t.Fatalf("scaffoldProject failed: %v", err)
	}

	loader := storage.NewLoader(storage.NewPaths(dir))

	proj, err := loader.LoadProject()
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if proj.Name != "hot-novel" {
		t.Errorf("project name: got %q", proj.Name)
	}
	if proj.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("created_at: got %q", proj.CreatedAt)
	}
	if proj.Story.Title != "城郊来电" || proj.Story.Premise != "一个外卖骑手的悬疑故事" {
		t.Errorf("story config mismatch: %+v", proj.Story)
	}
	if proj.Story.Language != "zh" {
		t.Errorf("language: got %q", proj.Story.Language)
	}

	char, err := loader.LoadProtagonist()
	if err != nil {
		t.Fatalf("load protagonist: %v", err)
	}
	if char.Name != "陈野" {
		t.Errorf("protagonist name: got %q", char.Name)
	}
	if len(char.Beliefs) != 1 || char.Beliefs[0] != "妻子的失踪是意外" {
		t.Errorf("beliefs not seeded: %+v", char.Beliefs)
	}
	if len(char.Values) != 2 {
		t.Errorf("values not seeded: %+v", char.Values)
	}

	world, err := loader.LoadWorld()
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	if world.CurrentNarrativeTime != "第0天，序幕" {
		t.Errorf("narrative time: got %q", world.CurrentNarrativeTime)
	}
}

func TestScaffoldProjectRefusesExistingProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte("x: y\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := scaffoldProject(dir, templateData{Name: "dup"}); err == nil {
		t.Fatal("expected error when project.yaml already exists")
	}
}
