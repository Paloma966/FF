package storage

import (
	"fmt"
	"os"

	"github.com/Paloma966/FF/internal/models"
	"github.com/Paloma966/FF/pkg/yamlutil"
)

// Saver handles persisting project data to disk.
type Saver struct {
	paths *Paths
}

// NewSaver creates a new Saver.
func NewSaver(paths *Paths) *Saver {
	return &Saver{paths: paths}
}

// SaveProject writes the project configuration.
func (s *Saver) SaveProject(proj *models.Project) error {
	return yamlutil.WriteFile(s.paths.ProjectYAML(), proj)
}

// SaveWorld writes the world state.
func (s *Saver) SaveWorld(world *models.WorldState) error {
	return yamlutil.WriteFile(s.paths.WorldYAML(), world)
}

// SaveProtagonist writes the protagonist character.
func (s *Saver) SaveProtagonist(char *models.Character) error {
	return yamlutil.WriteFile(s.paths.ProtagonistYAML(), char)
}

// SaveEvent writes an event to disk in the correct chapter directory.
func (s *Saver) SaveEvent(evt *models.Event) error {
	chapterDir := s.paths.ChapterDir(evt.ChapterNum)
	if err := os.MkdirAll(chapterDir, 0755); err != nil {
		return fmt.Errorf("create chapter dir: %w", err)
	}
	eventPath := s.paths.EventPath(evt.ChapterNum, evt.ID)
	return yamlutil.WriteFile(eventPath, evt)
}

// SaveChapterMarkdown writes a generated chapter to the generated directory.
func (s *Saver) SaveChapterMarkdown(chapterNum int, content string) error {
	genDir := s.paths.GeneratedDir()
	if err := os.MkdirAll(genDir, 0755); err != nil {
		return fmt.Errorf("create generated dir: %w", err)
	}
	path := s.paths.ChapterMarkdown(chapterNum)
	return os.WriteFile(path, []byte(content), 0644)
}

// EnsureDirs creates all necessary project directories.
func (s *Saver) EnsureDirs() error {
	dirs := []string{
		s.paths.CharactersDir(),
		s.paths.TimelineDir(),
		s.paths.GeneratedDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

// LockProject creates a state lock file to prevent concurrent runs.
func (s *Saver) LockProject() error {
	return os.WriteFile(s.paths.StateLock(), []byte("locked"), 0644)
}

// UnlockProject removes the state lock file.
func (s *Saver) UnlockProject() error {
	path := s.paths.StateLock()
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}

// IsLocked checks if the project is currently locked.
func (s *Saver) IsLocked() bool {
	_, err := os.Stat(s.paths.StateLock())
	return err == nil
}
