package check

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chun/fiction_factory/internal/models"
)

// CheckTimeline verifies event ordering and detects temporal/participant conflicts.
func CheckTimeline(protagonistName string, events []models.Event) []Issue {
	var issues []Issue

	if len(events) < 2 {
		return issues
	}

	// Sort events by chapter and ID (natural order)
	sorted := make([]models.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ChapterNum != sorted[j].ChapterNum {
			return sorted[i].ChapterNum < sorted[j].ChapterNum
		}
		return sorted[i].ID < sorted[j].ID
	})

	// Check for non-sequential chapter numbers
	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]

		// Chapter numbers should be sequential or same (multiple events per chapter)
		gap := curr.ChapterNum - prev.ChapterNum
		if gap > 1 {
			issues = append(issues, Issue{
				Severity: SeverityWarning,
				Category: "timeline",
				Message: fmt.Sprintf(
					"章节断层：第 %d 章 → 第 %d 章（第 %d 章没有事件）",
					prev.ChapterNum, curr.ChapterNum, prev.ChapterNum+1,
				),
				RelevantEvents: []string{prev.ID, curr.ID},
			})
		}

		// Chapter numbers should never decrease
		if gap < 0 {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Category: "timeline",
				Message: fmt.Sprintf(
					"章节顺序违规：事件 %s（第%d章）出现在事件 %s（第%d章）之后",
					curr.ID, curr.ChapterNum, prev.ID, prev.ChapterNum,
				),
				RelevantEvents: []string{prev.ID, curr.ID},
			})
		}
	}

	// Check narrative time progression (basic string check)
	type timeEntry struct {
		time  string
		event string
	}
	var times []timeEntry
	for _, e := range sorted {
		times = append(times, timeEntry{time: e.Time, event: e.ID})
	}

	// Look for identical time labels that suggest the events might be out of order
	seen := make(map[string]string)
	for _, t := range times {
		if prevEvent, ok := seen[t.time]; ok {
			// Two events at the same time — not necessarily an error but worth noting
			issues = append(issues, Issue{
				Severity: SeverityInfo,
				Category: "timeline",
				Message: fmt.Sprintf(
					"事件 %s 与 %s 使用了相同的叙事时间 '%s'",
					prevEvent, t.event, t.time,
				),
				RelevantEvents: []string{prevEvent, t.event},
			})
		}
		seen[t.time] = t.event
	}

	// Check that first event references protagonist if they're the main character
	for _, e := range events {
		found := false
		for _, p := range e.Participants {
			if strings.EqualFold(p, protagonistName) {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, Issue{
				Severity: SeverityInfo,
				Category: "timeline",
				Message: fmt.Sprintf(
					"事件 %s（%s）的参与者中没有主角 '%s'",
					e.ID, e.Title, protagonistName,
				),
				RelevantEvents: []string{e.ID},
			})
		}
	}

	return issues
}
