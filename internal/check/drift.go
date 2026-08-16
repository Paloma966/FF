package check

import (
	"fmt"

	"github.com/chun/fiction_factory/internal/models"
)

// CheckCharacterDrift detects unmotivated belief reversals and personality inconsistencies.
func CheckCharacterDrift(char *models.Character, events []models.Event) []Issue {
	var issues []Issue

	// Build a timeline of belief states
	type beliefState struct {
		belief string
		event  string
		action string // "formed", "modified", "abandoned"
	}

	beliefTimeline := make(map[string][]beliefState) // keyed by simplified belief text

	for _, evt := range events {
		for _, bc := range evt.BeliefChanges {
			if bc.Character != char.Name {
				continue
			}
			// Track the "after" belief being formed or modified
			key := simplifyBelief(bc.After)
			beliefTimeline[key] = append(beliefTimeline[key], beliefState{
				belief: bc.After,
				event:  evt.ID,
				action: "formed",
			})
			// Track the "before" belief being modified or abandoned
			keyBefore := simplifyBelief(bc.Before)
			beliefTimeline[keyBefore] = append(beliefTimeline[keyBefore], beliefState{
				belief: bc.Before,
				event:  evt.ID,
				action: "modified",
			})
		}

		// Check protagonist response for abandoned beliefs
		if evt.ProtagonistResponse != nil {
			for _, b := range evt.ProtagonistResponse.AbandonedBeliefs {
				key := simplifyBelief(b)
				beliefTimeline[key] = append(beliefTimeline[key], beliefState{
					belief: b,
					event:  evt.ID,
					action: "abandoned",
				})
			}
		}
	}

	// Detect reversals: belief appears → disappears → reappears without intervening event
	for key, states := range beliefTimeline {
		if len(states) < 2 {
			continue
		}

		var lastAbandonedAt string
		for _, s := range states {
			if s.action == "abandoned" {
				lastAbandonedAt = s.event
			}
			if s.action == "formed" && lastAbandonedAt != "" {
				issues = append(issues, Issue{
					Severity: SeverityWarning,
					Category: "drift",
					Message: fmt.Sprintf(
						"信念 '%s' 在 %s 中被抛弃，之后又在没有明确重建事件的情况下重新形成",
						key, lastAbandonedAt,
					),
					RelevantEvents: []string{lastAbandonedAt, s.event},
				})
				lastAbandonedAt = "" // Reset so we don't flag the same reappearance multiple times
			}
		}
	}

	// Check current beliefs against abandoned list
	for _, current := range char.Beliefs {
		key := simplifyBelief(current)
		for _, abandoned := range char.AbandonedBeliefs {
			if simplifyBelief(abandoned) == key {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Category: "drift",
					Message: fmt.Sprintf(
						"信念 '%s' 同时存在于活跃信念与已抛弃信念列表中",
						current,
					),
				})
			}
		}
	}

	return issues
}

// simplifyBelief reduces a belief string to a comparable key.
func simplifyBelief(belief string) string {
	// Remove common prefixes/suffixes for comparison
	s := belief
	prefixes := []string{"believes ", "suspects ", "wonders if ", "fears that "}
	for _, p := range prefixes {
		if len(s) > len(p) {
			prefix := s[:len(p)]
			if prefix == p {
				s = s[len(p):]
				break
			}
		}
	}
	return s
}
