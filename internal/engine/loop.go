package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/chun/fiction_factory/internal/llm"
	"github.com/chun/fiction_factory/internal/models"
	"github.com/chun/fiction_factory/internal/storage"
)

// RunLoop orchestrates one iteration of the story production cycle:
// Director → [Protagonist verifies / Director revises] → Protagonist →
// [Director verifies / Protagonist revises] → Chapter → Save.
type RunLoop struct {
	director    *DirectorAgent
	protagonist *ProtagonistAgent
	chapter     *ChapterGenerator
	premise     *PremiseGenerator
	loader      *storage.Loader
	saver       *storage.Saver
}

// LoopConfig configures the RunLoop with potentially different LLM clients per agent.
type LoopConfig struct {
	DirectorLLM    llm.LLMClient
	ProtagonistLLM llm.LLMClient
	ChapterLLM     llm.LLMClient
	PremiseLLM     llm.LLMClient
	Temperature    float64
	MaxTokens      int
	Language       string
}

// NewRunLoop creates a new RunLoop.
func NewRunLoop(loader *storage.Loader, saver *storage.Saver, cfg LoopConfig) *RunLoop {
	director := NewDirectorAgent(cfg.DirectorLLM)
	protagonist := NewProtagonistAgent(cfg.ProtagonistLLM)
	chapter := NewChapterGenerator(cfg.ChapterLLM)
	premise := NewPremiseGenerator(cfg.PremiseLLM)

	director.setLLMSettings(cfg.Temperature, cfg.MaxTokens)
	protagonist.setLLMSettings(cfg.Temperature, cfg.MaxTokens)
	chapter.setLLMSettings(cfg.Temperature, cfg.MaxTokens)

	director.setLanguage(cfg.Language)
	protagonist.setLanguage(cfg.Language)
	chapter.setLanguage(cfg.Language)

	return &RunLoop{
		director:    director,
		protagonist: protagonist,
		chapter:     chapter,
		premise:     premise,
		loader:      loader,
		saver:       saver,
	}
}

// RunResult contains all outputs from a single run iteration.
type RunResult struct {
	Event    *models.Event
	Analysis *DirectorOutput
	Reaction *models.ProtagonistReaction
	Chapter  string
}

// ChapterResult is one finished chapter plus its verification traces.
type ChapterResult struct {
	RunResult
	EventTrace    VerifyTrace
	ReactionTrace VerifyTrace
}

// NovelResult is the outcome of a full-novel run.
type NovelResult struct {
	Chapters []ChapterResult
}

// loadStoryState loads world, protagonist, and recent events in one go.
func (rl *RunLoop) loadStoryState() (*models.WorldState, *models.Character, []models.Event, error) {
	world, err := rl.loader.LoadWorld()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load world: %w", err)
	}
	protagonist, err := rl.loader.LoadProtagonist()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load protagonist: %w", err)
	}
	recentEvents, err := rl.loader.LoadRecentEvents(5)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load recent events: %w", err)
	}
	return world, protagonist, recentEvents, nil
}

// POV returns the project's point of view setting.
func (rl *RunLoop) pov() string {
	proj, err := rl.loader.LoadProject()
	if err != nil {
		return "third_person_limited"
	}
	return proj.Story.POV
}

// tense returns the project's tense setting.
func (rl *RunLoop) tense() string {
	proj, err := rl.loader.LoadProject()
	if err != nil {
		return "past"
	}
	return proj.Story.Tense
}

// language returns the project's novel language ("zh" by default).
func (rl *RunLoop) language() string {
	proj, err := rl.loader.LoadProject()
	if err != nil || proj.Story.Language == "" {
		return "zh"
	}
	return proj.Story.Language
}

// Step1_DirectorProposal runs the Director Agent and returns proposed events.
func (rl *RunLoop) Step1_DirectorProposal(ctx context.Context) (*models.Event, *DirectorOutput, error) {
	world, protagonist, recentEvents, err := rl.loadStoryState()
	if err != nil {
		return nil, nil, err
	}

	nextChapter, err := rl.loader.NextChapterNum()
	if err != nil {
		return nil, nil, fmt.Errorf("get next chapter: %w", err)
	}

	// Build state
	state := DirectorState{
		Facts:            world.Facts,
		Threads:          world.UnresolvedThreads(),
		Protagonist:      protagonist,
		RecentMemories:   protagonist.RecentMemories(5),
		RecentEvents:     recentEvents,
		RecentEventCount: len(recentEvents),
		NextChapterNum:   nextChapter,
		CurrentTime:      world.CurrentNarrativeTime,
	}

	return rl.director.ProposeEvent(ctx, state)
}

// Step1_Verified runs the Director proposal, then has the Protagonist verify
// it. On rejection the Director revises the event, up to `rounds` revisions,
// after which the latest version is accepted anyway (Forced=true).
func (rl *RunLoop) Step1_Verified(ctx context.Context, rounds int) (*models.Event, *DirectorOutput, VerifyTrace, error) {
	event, analysis, err := rl.Step1_DirectorProposal(ctx)
	if err != nil {
		return nil, nil, VerifyTrace{}, err
	}

	trace := VerifyTrace{}
	for i := 0; i <= rounds; i++ {
		world, protagonist, recentEvents, err := rl.loadStoryState()
		if err != nil {
			return nil, nil, trace, err
		}
		verdict, err := rl.protagonist.VerifyEvent(ctx, VerifyEventInput{
			Character:    protagonist,
			Facts:        world.Facts,
			Threads:      world.UnresolvedThreads(),
			RecentEvents: recentEvents,
			Event:        event,
		})
		if err != nil {
			return nil, nil, trace, fmt.Errorf("verify event: %w", err)
		}
		if verdict.Approved {
			trace.Revisions = i
			return event, analysis, trace, nil
		}
		if i == rounds {
			trace.Revisions = rounds
			trace.Forced = true
			trace.Issues = verdict.Issues
			return event, analysis, trace, nil
		}
		trace.Issues = verdict.Issues
		event, err = rl.director.ReviseEvent(ctx, event, verdict)
		if err != nil {
			return nil, nil, trace, fmt.Errorf("revise event: %w", err)
		}
	}
	// Unreachable, but keep the compiler happy.
	return event, analysis, trace, nil
}

// Step2_ProtagonistResponse runs the Protagonist Agent on an event.
func (rl *RunLoop) Step2_ProtagonistResponse(ctx context.Context, event *models.Event) (*models.ProtagonistReaction, error) {
	protagonist, err := rl.loader.LoadProtagonist()
	if err != nil {
		return nil, fmt.Errorf("load protagonist: %w", err)
	}

	state := ProtagonistState{
		Name:           protagonist.Name,
		Beliefs:        protagonist.Beliefs,
		Goals:          protagonist.Goals,
		Fears:          protagonist.Fears,
		Values:         protagonist.Values,
		RecentMemories: protagonist.RecentMemories(5),
		Event: ProcessedEvent{
			Title:          event.Title,
			Time:           event.Time,
			Tone:           event.Tone,
			Description:    event.Description,
			FactsChanged:   event.FactsChanged,
			Participants:   event.Participants,
			DirectorIntent: event.DirectorIntent,
		},
	}

	return rl.protagonist.ProcessEvent(ctx, state)
}

// Step2_Verified runs the Protagonist response, then has the Director verify
// it. On rejection the Protagonist revises the reaction, up to `rounds`
// revisions, after which the latest version is accepted anyway (Forced=true).
func (rl *RunLoop) Step2_Verified(ctx context.Context, event *models.Event, rounds int) (*models.ProtagonistReaction, VerifyTrace, error) {
	reaction, err := rl.Step2_ProtagonistResponse(ctx, event)
	if err != nil {
		return nil, VerifyTrace{}, err
	}

	trace := VerifyTrace{}
	for i := 0; i <= rounds; i++ {
		world, protagonist, _, err := rl.loadStoryState()
		if err != nil {
			return nil, trace, err
		}
		verdict, err := rl.director.VerifyReaction(ctx, VerifyReactionInput{
			Event:     event,
			Character: protagonist,
			Facts:     world.Facts,
			Reaction:  reaction,
		})
		if err != nil {
			return nil, trace, fmt.Errorf("verify reaction: %w", err)
		}
		if verdict.Approved {
			trace.Revisions = i
			return reaction, trace, nil
		}
		if i == rounds {
			trace.Revisions = rounds
			trace.Forced = true
			trace.Issues = verdict.Issues
			return reaction, trace, nil
		}
		trace.Issues = verdict.Issues
		reaction, err = rl.protagonist.ReviseReaction(ctx, event, reaction, verdict)
		if err != nil {
			return nil, trace, fmt.Errorf("revise reaction: %w", err)
		}
	}
	return reaction, trace, nil
}

// Step3_GenerateChapter generates the chapter prose.
func (rl *RunLoop) Step3_GenerateChapter(ctx context.Context, event *models.Event, reaction *models.ProtagonistReaction, prevSummary string) (string, error) {
	input := ChapterInputFromEvent(event, reaction, rl.pov(), rl.tense(), prevSummary)

	// Add chapter number header if not present
	chapter, err := rl.chapter.Generate(ctx, input)
	if err != nil {
		return "", err
	}

	// Ensure chapter has a title
	if !strings.HasPrefix(chapter, "#") {
		if rl.language() == "zh" {
			chapter = fmt.Sprintf("# 第%d章：%s\n\n%s", event.ChapterNum, event.Title, chapter)
		} else {
			chapter = fmt.Sprintf("# Chapter %d: %s\n\n%s", event.ChapterNum, event.Title, chapter)
		}
	}

	return chapter, nil
}

// SaveAll persists all outputs from a run iteration.
func (rl *RunLoop) SaveAll(event *models.Event, reaction *models.ProtagonistReaction, chapterText string) error {
	// Set IDs and chapter numbers
	world, err := rl.loader.LoadWorld()
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}

	eventID, err := rl.loader.NextEventID()
	if err != nil {
		return fmt.Errorf("next event ID: %w", err)
	}

	event.ID = eventID
	event.ChapterNum = world.ChapterCount + 1
	event.Lifecycle = models.EventResolved
	event.ProtagonistResponse = reaction
	event.ChapterText = chapterText

	// Fill in derived FutureHook fields
	for i := range event.FutureHooks {
		event.FutureHooks[i].PlantedIn = eventID
	}

	// Save event
	if err := rl.saver.SaveEvent(event); err != nil {
		return fmt.Errorf("save event: %w", err)
	}

	// Update and save world state
	rl.applyFactsToWorld(world, event)
	world.Threads = append(world.Threads, event.FutureHooks...)
	rl.markResolvedHooks(world, event.ResolvesHooks, event.ID)
	world.ChapterCount = event.ChapterNum
	world.EventCount++
	world.CurrentNarrativeTime = event.Time

	if err := rl.saver.SaveWorld(world); err != nil {
		return fmt.Errorf("save world: %w", err)
	}

	// Update and save protagonist
	protagonist, err := rl.loader.LoadProtagonist()
	if err != nil {
		return fmt.Errorf("load protagonist: %w", err)
	}
	ApplyReaction(protagonist, eventID, reaction, event.Time)
	if err := rl.saver.SaveProtagonist(protagonist); err != nil {
		return fmt.Errorf("save protagonist: %w", err)
	}

	// Save chapter markdown
	if err := rl.saver.SaveChapterMarkdown(event.ChapterNum, chapterText); err != nil {
		return fmt.Errorf("save chapter: %w", err)
	}

	return nil
}

// applyFactsToWorld updates world facts based on event's FactsChanged.
func (rl *RunLoop) applyFactsToWorld(world *models.WorldState, event *models.Event) {
	for _, fc := range event.FactsChanged {
		// Remove old fact
		world.Facts = removeString(world.Facts, fc.Before)
		// Add new fact
		world.Facts = append(world.Facts, fc.After)
	}
}

// markResolvedHooks marks hooks as resolved in the world state, recording the
// event ID that resolved them.
func (rl *RunLoop) markResolvedHooks(world *models.WorldState, resolvedIDs []string, resolvedIn string) {
	for _, id := range resolvedIDs {
		for i := range world.Threads {
			if world.Threads[i].ID == id {
				world.Threads[i].ResolvedIn = resolvedIn
			}
		}
	}
}

// NovelConfig configures a full-novel production run.
type NovelConfig struct {
	Chapters     int                               // number of chapters to generate
	VerifyRounds int                               // max revision rounds per verification
	Progress     func(ch ChapterResult, total int) // called after each chapter
}

// RunNovel produces a complete novel: one verified event + reaction + chapter
// per iteration, saving everything to the project directory.
func (rl *RunLoop) RunNovel(ctx context.Context, cfg NovelConfig) (*NovelResult, error) {
	chapters := cfg.Chapters
	if chapters <= 0 {
		chapters = 10
	}
	rounds := cfg.VerifyRounds
	if rounds < 0 {
		rounds = 0
	}

	result := &NovelResult{}
	var prevSummary string

	for ch := 1; ch <= chapters; ch++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		event, _, eventTrace, err := rl.Step1_Verified(ctx, rounds)
		if err != nil {
			return result, fmt.Errorf("chapter %d director: %w", ch, err)
		}

		reaction, reactionTrace, err := rl.Step2_Verified(ctx, event, rounds)
		if err != nil {
			return result, fmt.Errorf("chapter %d protagonist: %w", ch, err)
		}

		chapterText, err := rl.Step3_GenerateChapter(ctx, event, reaction, prevSummary)
		if err != nil {
			return result, fmt.Errorf("chapter %d prose: %w", ch, err)
		}

		if err := rl.SaveAll(event, reaction, chapterText); err != nil {
			return result, fmt.Errorf("chapter %d save: %w", ch, err)
		}

		prevSummary = truncateSummary(chapterText, 600)

		cr := ChapterResult{
			RunResult: RunResult{
				Event:    event,
				Reaction: reaction,
				Chapter:  chapterText,
			},
			EventTrace:    eventTrace,
			ReactionTrace: reactionTrace,
		}
		result.Chapters = append(result.Chapters, cr)
		if cfg.Progress != nil {
			cfg.Progress(cr, chapters)
		}
	}

	return result, nil
}

// truncateSummary returns the first maxRunes runes of text (for continuity
// context in the next chapter's prompt).
func truncateSummary(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

// RunFull executes one legacy interactive cycle and saves everything.
func (rl *RunLoop) RunFull(ctx context.Context) (*RunResult, error) {
	// Step 1: Director
	fmt.Println("\n🎬 Director is analyzing the story...")
	event, analysis, err := rl.Step1_DirectorProposal(ctx)
	if err != nil {
		return nil, fmt.Errorf("director step: %w", err)
	}

	// Step 2: Protagonist
	fmt.Println("🎭 Protagonist is processing the event...")
	reaction, err := rl.Step2_ProtagonistResponse(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("protagonist step: %w", err)
	}

	// Step 3: Chapter
	fmt.Println("📝 Generating chapter prose...")
	chapter, err := rl.Step3_GenerateChapter(ctx, event, reaction, "")
	if err != nil {
		return nil, fmt.Errorf("chapter step: %w", err)
	}

	// Save
	fmt.Println("💾 Saving...")
	if err := rl.SaveAll(event, reaction, chapter); err != nil {
		return nil, fmt.Errorf("save step: %w", err)
	}

	return &RunResult{
		Event:    event,
		Analysis: analysis,
		Reaction: reaction,
		Chapter:  chapter,
	}, nil
}
