package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/models"
	"github.com/chun/fiction_factory/internal/storage"
)

const chapterProse = "# 第一章 测试标题\n\n正文内容。林恩打开了那封信。\n"

// scaffoldTestProject creates a minimal on-disk project for loop tests.
func scaffoldTestProject(t *testing.T, dir string) (*storage.Loader, *storage.Saver) {
	t.Helper()
	paths := storage.NewPaths(dir)
	saver := storage.NewSaver(paths)
	if err := saver.EnsureDirs(); err != nil {
		t.Fatal(err)
	}

	proj := &models.Project{
		Name:      "test",
		CreatedAt: "2026-07-30T00:00:00Z",
		Story: models.StoryConfig{
			Title:    "测试小说",
			Genre:    "都市悬疑",
			Premise:  "测试梗概",
			POV:      "third_person_limited",
			Tense:    "past",
			Language: "zh",
		},
		Protagonist: models.ProtagonistConfig{Name: "林恩"},
		LLM: models.LLMConfig{
			Provider:    "mock",
			Model:       "mock",
			Temperature: 0.8,
			MaxTokens:   4096,
		},
	}
	if err := saver.SaveProject(proj); err != nil {
		t.Fatal(err)
	}

	world := &models.WorldState{
		Facts:                []string{},
		Threads:              nil,
		ChapterCount:         0,
		EventCount:           0,
		CurrentNarrativeTime: "第0天，序幕",
	}
	if err := saver.SaveWorld(world); err != nil {
		t.Fatal(err)
	}

	char := &models.Character{
		Name:    "林恩",
		Beliefs: []string{"believes the world is simple"},
	}
	if err := saver.SaveProtagonist(char); err != nil {
		t.Fatal(err)
	}

	return storage.NewLoader(paths), saver
}

func newMockLoop(t *testing.T, dir string, responses ...string) (*RunLoop, *storage.Loader) {
	t.Helper()
	loader, saver := scaffoldTestProject(t, dir)
	mock := llm.NewMockLLM().WithResponses(responses...)
	loop := NewRunLoop(loader, saver, LoopConfig{
		DirectorLLM:    mock,
		ProtagonistLLM: mock,
		ChapterLLM:     mock,
		PremiseLLM:     mock,
	})
	return loop, loader
}

func TestRunNovel_OneChapterApproved(t *testing.T) {
	dir := t.TempDir()
	loop, loader := newMockLoop(t, dir,
		validDirectorYAML(),    // 1: director proposal
		approvedVerdict,        // 2: protagonist verifies event
		validProtagonistYAML(), // 3: protagonist reaction
		approvedVerdict,        // 4: director verifies reaction
		chapterProse,           // 5: chapter prose
	)

	result, err := loop.RunNovel(context.Background(), NovelConfig{Chapters: 1, VerifyRounds: 0})
	if err != nil {
		t.Fatalf("RunNovel failed: %v", err)
	}
	if len(result.Chapters) != 1 {
		t.Fatalf("expected 1 chapter, got %d", len(result.Chapters))
	}
	cr := result.Chapters[0]
	if cr.Event.ID != "evt-001" {
		t.Errorf("event ID: got %q", cr.Event.ID)
	}
	if cr.EventTrace.Forced || cr.EventTrace.Revisions != 0 {
		t.Errorf("unexpected event trace: %+v", cr.EventTrace)
	}
	if cr.ReactionTrace.Forced || cr.ReactionTrace.Revisions != 0 {
		t.Errorf("unexpected reaction trace: %+v", cr.ReactionTrace)
	}

	// Disk state
	world, err := loader.LoadWorld()
	if err != nil {
		t.Fatal(err)
	}
	if world.ChapterCount != 1 || world.EventCount != 1 {
		t.Errorf("world counts: chapter=%d event=%d", world.ChapterCount, world.EventCount)
	}
	char, err := loader.LoadProtagonist()
	if err != nil {
		t.Fatal(err)
	}
	if len(char.Memories) != 1 {
		t.Errorf("expected 1 memory, got %d", len(char.Memories))
	}
	md, err := os.ReadFile(filepath.Join(dir, "generated", "chapter-01.md"))
	if err != nil {
		t.Fatalf("chapter file: %v", err)
	}
	if string(md) != chapterProse {
		t.Errorf("chapter content mismatch")
	}
}

func TestRunNovel_RevisionsThenApproved(t *testing.T) {
	dir := t.TempDir()
	loop, _ := newMockLoop(t, dir,
		validDirectorYAML(),    // 1: propose
		rejectedVerdict,        // 2: verify → reject
		revisedEventYAML,       // 3: director revises
		approvedVerdict,        // 4: verify → approve
		validProtagonistYAML(), // 5: reaction
		rejectedVerdict,        // 6: verify → reject
		revisedReactionYAML(),  // 7: protagonist revises
		approvedVerdict,        // 8: verify → approve
		chapterProse,           // 9: chapter
	)

	result, err := loop.RunNovel(context.Background(), NovelConfig{Chapters: 1, VerifyRounds: 1})
	if err != nil {
		t.Fatalf("RunNovel failed: %v", err)
	}
	cr := result.Chapters[0]
	if cr.EventTrace.Forced || cr.EventTrace.Revisions != 1 {
		t.Errorf("expected 1 revision then approve, got %+v", cr.EventTrace)
	}
	if cr.ReactionTrace.Forced || cr.ReactionTrace.Revisions != 1 {
		t.Errorf("expected 1 revision then approve, got %+v", cr.ReactionTrace)
	}
	if cr.Event.Title != "修订后的秘密信件" {
		t.Errorf("expected revised event title, got %q", cr.Event.Title)
	}
}

func TestRunNovel_ForcedAccept(t *testing.T) {
	dir := t.TempDir()
	loop, _ := newMockLoop(t, dir,
		validDirectorYAML(),    // 1: propose
		rejectedVerdict,        // 2: verify → reject
		revisedEventYAML,       // 3: revise
		rejectedVerdict,        // 4: verify → reject → forced
		validProtagonistYAML(), // 5: reaction
		rejectedVerdict,        // 6: verify → reject
		revisedReactionYAML(),  // 7: revise
		rejectedVerdict,        // 8: verify → reject → forced
		chapterProse,           // 9: chapter
	)

	result, err := loop.RunNovel(context.Background(), NovelConfig{Chapters: 1, VerifyRounds: 1})
	if err != nil {
		t.Fatalf("RunNovel failed: %v", err)
	}
	cr := result.Chapters[0]
	if !cr.EventTrace.Forced || cr.EventTrace.Revisions != 1 {
		t.Errorf("expected forced accept after 1 revision, got %+v", cr.EventTrace)
	}
	if len(cr.EventTrace.Issues) != 2 {
		t.Errorf("expected issues recorded, got %v", cr.EventTrace.Issues)
	}
	if !cr.ReactionTrace.Forced {
		t.Errorf("expected forced reaction trace, got %+v", cr.ReactionTrace)
	}
}
