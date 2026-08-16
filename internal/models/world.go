package models

// WorldState stores the current state of the story world.
type WorldState struct {
	Facts                []string     `yaml:"facts"`
	Threads              []FutureHook `yaml:"threads"`
	ChapterCount         int          `yaml:"chapter_count"`
	EventCount           int          `yaml:"event_count"`
	CurrentNarrativeTime string       `yaml:"current_narrative_time"`
}

// UnresolvedThreads returns hooks that have not been resolved.
func (w *WorldState) UnresolvedThreads() []FutureHook {
	var unresolved []FutureHook
	for _, t := range w.Threads {
		if t.ResolvedIn == "" {
			unresolved = append(unresolved, t)
		}
	}
	return unresolved
}

// ResolvedThreads returns hooks that have been resolved.
func (w *WorldState) ResolvedThreads() []FutureHook {
	var resolved []FutureHook
	for _, t := range w.Threads {
		if t.ResolvedIn != "" {
			resolved = append(resolved, t)
		}
	}
	return resolved
}
