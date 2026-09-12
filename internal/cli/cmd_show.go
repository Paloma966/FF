package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Paloma966/FF/internal/models"
	"github.com/Paloma966/FF/internal/storage"
	"github.com/spf13/cobra"
)

var (
	showDetail bool
	showHooks  bool
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display world state, character status, and timeline",
	Long: `Load and display the current state of the project including:
  - World facts and unresolved plot threads
  - Protagonist beliefs, goals, fears, and recent memories
  - Timeline summary (events ordered by chapter and time)

Flags:
  --detail   Show full event details
  --hooks    Show only hook/thread status`,
	RunE: runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showDetail, "detail", false, "Show full event details")
	showCmd.Flags().BoolVar(&showHooks, "hooks", false, "Show only hook/thread status")
}

func runShow(cmd *cobra.Command, args []string) error {
	projectDir := GetProjectDir()
	paths := storage.NewPaths(projectDir)

	if _, err := os.Stat(paths.ProjectYAML()); os.IsNotExist(err) {
		return fmt.Errorf("no project found at %s — run 'ff init' first", projectDir)
	}

	loader := storage.NewLoader(paths)

	proj, err := loader.LoadProject()
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}

	world, err := loader.LoadWorld()
	if err != nil {
		return fmt.Errorf("load world: %w", err)
	}

	protagonist, err := loader.LoadProtagonist()
	if err != nil {
		return fmt.Errorf("load protagonist: %w", err)
	}

	events, err := loader.LoadTimeline()
	if err != nil {
		return fmt.Errorf("load timeline: %w", err)
	}

	// If --hooks flag, only show hooks
	if showHooks {
		displayHooks(world, events)
		return nil
	}

	// Header
	fmt.Println()
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("  📖 %s\n", proj.Story.Title)
	if proj.Story.Premise != "" {
		fmt.Printf("     %s\n", proj.Story.Premise)
	}
	fmt.Println(strings.Repeat("═", 60))

	// Project overview
	fmt.Println()
	fmt.Printf("📊 PROJECT OVERVIEW\n")
	fmt.Printf("  Chapters: %d  |  Events: %d  |  POV: %s  |  Tense: %s  |  Language: %s\n",
		world.ChapterCount, world.EventCount, proj.Story.POV, proj.Story.Tense, proj.Story.Language)
	fmt.Printf("  Narrative Time: %s\n", world.CurrentNarrativeTime)

	// World Facts
	fmt.Println()
	fmt.Printf("🌍 WORLD FACTS (%d)\n", len(world.Facts))
	fmt.Println(strings.Repeat("─", 40))
	if len(world.Facts) == 0 {
		fmt.Println("  (No world facts established yet)")
	} else {
		for _, fact := range world.Facts {
			fmt.Printf("  • %s\n", fact)
		}
	}

	// Protagonist State
	fmt.Println()
	fmt.Printf("🎭 PROTAGONIST: %s\n", protagonist.Name)
	fmt.Println(strings.Repeat("─", 40))

	displayList("Beliefs", protagonist.Beliefs, "💭")
	displayList("Goals", protagonist.Goals, "🎯")
	displayList("Fears", protagonist.Fears, "😰")
	displayList("Values", protagonist.Values, "⚖️")

	// Recent Memories
	if len(protagonist.Memories) > 0 {
		fmt.Println()
		fmt.Printf("🧠 RECENT MEMORIES (%d total)\n", len(protagonist.Memories))
		fmt.Println(strings.Repeat("─", 40))
		recent := protagonist.RecentMemories(5)
		for _, m := range recent {
			fmt.Printf("  [%s] %s\n", m.EventID, m.Content)
			fmt.Printf("         intensity: %d/10 | category: %s\n", m.Intensity, m.Category)
		}
	}

	// Timeline
	fmt.Println()
	fmt.Printf("📜 TIMELINE (%d events)\n", len(events))
	fmt.Println(strings.Repeat("─", 40))
	if len(events) == 0 {
		fmt.Println("  (No events yet — run 'ff run' to generate the first chapter)")
	} else {
		// Group by chapter
		type chapterGroup struct {
			num    int
			events []models.Event
		}
		chapters := make(map[int]*chapterGroup)
		for _, e := range events {
			if _, ok := chapters[e.ChapterNum]; !ok {
				chapters[e.ChapterNum] = &chapterGroup{num: e.ChapterNum}
			}
			chapters[e.ChapterNum].events = append(chapters[e.ChapterNum].events, e)
		}

		var sortedChapters []int
		for n := range chapters {
			sortedChapters = append(sortedChapters, n)
		}
		sort.Ints(sortedChapters)

		for _, chNum := range sortedChapters {
			ch := chapters[chNum]
			fmt.Printf("\n  Chapter %d (%d events)\n", ch.num, len(ch.events))
			for _, e := range ch.events {
				status := lifecycleIcon(e.Lifecycle)
				fmt.Printf("  %s %-6s %-4s %s\n", status, e.ID, fmt.Sprintf("T%d", e.TensionLevel), e.Title)
				if IsVerbose() || showDetail {
					fmt.Printf("     Time: %s | Tone: %s\n", e.Time, e.Tone)
					fmt.Printf("     %s\n", wrapSummary(e.Description, 56))
				}
			}
		}

		if showDetail {
			fmt.Println()
			fmt.Println(strings.Repeat("═", 60))
			fmt.Println("📋 FULL EVENT DETAILS")
			fmt.Println(strings.Repeat("═", 60))
			for _, e := range events {
				fmt.Println()
				displayEventDetail(e)
			}
		}
	}

	// Unresolved Hooks
	unresolved := world.UnresolvedThreads()
	resolved := world.ResolvedThreads()
	fmt.Println()
	fmt.Printf("🧵 PLOT THREADS (hooks)\n")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("  Unresolved: %d  |  Resolved: %d\n", len(unresolved), len(resolved))
	if len(unresolved) > 0 {
		fmt.Println()
		fmt.Println("  UNRESOLVED:")
		for _, hook := range unresolved {
			urgencyIcon := urgencySymbol(hook.Urgency)
			fmt.Printf("  %s [%s] %s\n", urgencyIcon, hook.ID, hook.Description)
			fmt.Printf("      Urgency: %s | Hints at: %s | Planted in: %s\n",
				hook.Urgency, hook.HintsAt, hook.PlantedIn)
		}
	}
	if len(resolved) > 0 {
		fmt.Println()
		fmt.Println("  RESOLVED:")
		for _, hook := range resolved {
			fmt.Printf("  ✓ [%s] %s\n", hook.ID, hook.Description)
		}
	}

	fmt.Println()
	return nil
}

func displayHooks(world *models.WorldState, events []models.Event) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 60))
	fmt.Println("🧵 PLOT THREAD STATUS")
	fmt.Println(strings.Repeat("═", 60))

	unresolved := world.UnresolvedThreads()
	resolved := world.ResolvedThreads()
	all := world.Threads

	fmt.Printf("\nTotal hooks: %d | Unresolved: %d | Resolved: %d\n\n",
		len(all), len(unresolved), len(resolved))

	// Sort by urgency
	sort.Slice(unresolved, func(i, j int) bool {
		order := map[string]int{"immediate": 0, "soon": 1, "eventual": 2, "dormant": 3}
		return order[unresolved[i].Urgency] < order[unresolved[j].Urgency]
	})

	if len(unresolved) > 0 {
		fmt.Println("UNRESOLVED (by urgency):")
		fmt.Println(strings.Repeat("─", 40))
		for _, hook := range unresolved {
			fmt.Printf("  %s [%s] %s\n", urgencySymbol(hook.Urgency), hook.ID, hook.Description)
			fmt.Printf("    Urgency: %s | Hints: %s | Planted: %s\n\n",
				hook.Urgency, hook.HintsAt, hook.PlantedIn)
		}
	}

	if len(resolved) > 0 {
		fmt.Println("RESOLVED:")
		fmt.Println(strings.Repeat("─", 40))
		for _, hook := range resolved {
			fmt.Printf("  ✓ [%s] %s (resolved in: %s)\n", hook.ID, hook.Description, hook.ResolvedIn)
		}
	}
	fmt.Println()
}

func displayList(label string, items []string, icon string) {
	if len(items) > 0 {
		fmt.Printf("\n  %s %s:\n", icon, label)
		for _, item := range items {
			fmt.Printf("    • %s\n", item)
		}
	}
}

func displayEventDetail(e models.Event) {
	fmt.Printf("  Event: %s — %s\n", e.ID, e.Title)
	fmt.Printf("  Chapter: %d | Time: %s | Lifecycle: %s | Tension: %d/10 | Tone: %s\n",
		e.ChapterNum, e.Time, e.Lifecycle, e.TensionLevel, e.Tone)
	fmt.Printf("  Participants: %s\n", strings.Join(e.Participants, ", "))
	fmt.Printf("  Director Intent: %s\n", e.DirectorIntent)
	fmt.Printf("  Description: %s\n", wrapSummary(e.Description, 56))

	if len(e.FactsChanged) > 0 {
		fmt.Println("  Facts Changed:")
		for _, fc := range e.FactsChanged {
			fmt.Printf("    Before: %s\n    After:  %s\n", fc.Before, fc.After)
		}
	}
	if len(e.BeliefChanges) > 0 {
		fmt.Println("  Belief Changes:")
		for _, bc := range e.BeliefChanges {
			fmt.Printf("    %s: \"%s\" → \"%s\"\n", bc.Character, bc.Before, bc.After)
		}
	}
	if len(e.FutureHooks) > 0 {
		fmt.Println("  Hooks Planted:")
		for _, h := range e.FutureHooks {
			fmt.Printf("    [%s] %s (urgency: %s)\n", h.ID, h.Description, h.Urgency)
		}
	}
	if len(e.ResolvesHooks) > 0 {
		fmt.Println("  Hooks Resolved:")
		for _, h := range e.ResolvesHooks {
			fmt.Printf("    ✓ %s\n", h)
		}
	}
	if len(e.CausedBy) > 0 {
		fmt.Println("  Caused By:")
		for _, ref := range e.CausedBy {
			fmt.Printf("    ← %s (%s)\n", ref.EventID, ref.Title)
		}
	}

	if e.ProtagonistResponse != nil {
		r := e.ProtagonistResponse
		fmt.Printf("  Protagonist Reaction:\n")
		fmt.Printf("    Emotional: %s\n", wrapSummary(r.EmotionalResponse, 52))
		fmt.Printf("    Decision: %s\n", wrapSummary(r.Decision, 52))
	}
	fmt.Println()
}

func lifecycleIcon(lc models.EventLifecycle) string {
	switch lc {
	case models.EventProposed:
		return "💡"
	case models.EventPlanned:
		return "📋"
	case models.EventExecuting:
		return "⚡"
	case models.EventResolved:
		return "✅"
	case models.EventAbandoned:
		return "🗑️"
	default:
		return "❓"
	}
}

func urgencySymbol(urgency string) string {
	switch urgency {
	case "immediate":
		return "🔴"
	case "soon":
		return "🟡"
	case "eventual":
		return "🟢"
	case "dormant":
		return "💤"
	default:
		return "⚪"
	}
}

func wrapSummary(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	return string(runes[:width-3]) + "..."
}
