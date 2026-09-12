package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Paloma966/FF/internal/models"
	"github.com/Paloma966/FF/pkg/yamlutil"
)

// Loader handles reading project data from disk.
type Loader struct {
	paths *Paths
}

// NewLoader creates a new Loader.
func NewLoader(paths *Paths) *Loader {
	return &Loader{paths: paths}
}

// LoadProject reads the project configuration.
func (l *Loader) LoadProject() (*models.Project, error) {
	var proj models.Project
	if err := yamlutil.ReadFile(l.paths.ProjectYAML(), &proj); err != nil {
		return nil, fmt.Errorf("load project: %w", err)
	}
	return &proj, nil
}

// LoadWorld reads the world state.
func (l *Loader) LoadWorld() (*models.WorldState, error) {
	var world models.WorldState
	if err := yamlutil.ReadFile(l.paths.WorldYAML(), &world); err != nil {
		return nil, fmt.Errorf("load world: %w", err)
	}
	return &world, nil
}

// LoadProtagonist reads the protagonist character.
func (l *Loader) LoadProtagonist() (*models.Character, error) {
	var char models.Character
	if err := yamlutil.ReadFile(l.paths.ProtagonistYAML(), &char); err != nil {
		return nil, fmt.Errorf("load protagonist: %w", err)
	}
	return &char, nil
}

// LoadTimeline reads all events from disk, sorted by chapter and event ID.
func (l *Loader) LoadTimeline() ([]models.Event, error) {
	timelineDir := l.paths.TimelineDir()
	entries, err := os.ReadDir(timelineDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read timeline dir: %w", err)
	}

	var events []models.Event
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "chapter-") {
			continue
		}
		chapterDir := filepath.Join(timelineDir, entry.Name())
		eventFiles, err := os.ReadDir(chapterDir)
		if err != nil {
			continue
		}
		for _, ef := range eventFiles {
			if ef.IsDir() || !strings.HasSuffix(ef.Name(), ".yaml") {
				continue
			}
			var evt models.Event
			eventPath := filepath.Join(chapterDir, ef.Name())
			if err := yamlutil.ReadFile(eventPath, &evt); err != nil {
				continue
			}
			events = append(events, evt)
		}
	}

	// Sort by chapter number, then by event ID
	sort.Slice(events, func(i, j int) bool {
		if events[i].ChapterNum != events[j].ChapterNum {
			return events[i].ChapterNum < events[j].ChapterNum
		}
		return events[i].ID < events[j].ID
	})

	return events, nil
}

// LoadRecentEvents loads the most recent N events from the timeline.
func (l *Loader) LoadRecentEvents(n int) ([]models.Event, error) {
	all, err := l.LoadTimeline()
	if err != nil {
		return nil, err
	}
	if len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}

// LoadChapterEvents loads all events for a specific chapter.
func (l *Loader) LoadChapterEvents(chapterNum int) ([]models.Event, error) {
	chapterDir := l.paths.ChapterDir(chapterNum)
	entries, err := os.ReadDir(chapterDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []models.Event
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		var evt models.Event
		if err := yamlutil.ReadFile(filepath.Join(chapterDir, entry.Name()), &evt); err != nil {
			continue
		}
		events = append(events, evt)
	}
	return events, nil
}

// NextChapterNum determines the next chapter number.
func (l *Loader) NextChapterNum() (int, error) {
	world, err := l.LoadWorld()
	if err != nil {
		return 0, err
	}
	return world.ChapterCount + 1, nil
}

// NextEventID generates the next event ID based on the current event count.
func (l *Loader) NextEventID() (string, error) {
	world, err := l.LoadWorld()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("evt-%03d", world.EventCount+1), nil
}

// parseChapterNum extracts the chapter number from a directory name like "chapter-03".
func parseChapterNum(dirName string) (int, error) {
	parts := strings.Split(dirName, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid chapter dir: %s", dirName)
	}
	return strconv.Atoi(parts[1])
}
