package check

import (
	"fmt"
	"strings"

	"github.com/chun/fiction_factory/internal/models"
)

// Severity indicates the seriousness of an issue.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARNING"
	SeverityInfo    Severity = "INFO"
)

// Issue represents a single consistency problem found.
type Issue struct {
	Severity       Severity
	Category       string // "drift", "timeline", "threads"
	Message        string
	RelevantEvents []string // Event IDs involved
}

// Report contains all issues found during a consistency check.
type Report struct {
	Issues []Issue
}

// HasErrors returns true if there are any ERROR-level issues.
func (r *Report) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any WARNING-level issues.
func (r *Report) HasWarnings() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// CountBySeverity returns counts for each severity level.
func (r *Report) CountBySeverity() (errors, warnings, infos int) {
	for _, i := range r.Issues {
		switch i.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		case SeverityInfo:
			infos++
		}
	}
	return
}

// Print displays the report in a readable format.
func (r *Report) Print() {
	if len(r.Issues) == 0 {
		fmt.Println("✅ 未发现一致性问题。")
		return
	}

	errors, warnings, infos := r.CountBySeverity()
	fmt.Println()
	fmt.Println(strings.Repeat("═", 60))
	fmt.Printf("📋 一致性检查结果\n")
	fmt.Printf("   错误: %d | 警告: %d | 提示: %d\n", errors, warnings, infos)
	fmt.Println(strings.Repeat("═", 60))

	// Print by category
	categories := []string{"drift", "timeline", "threads"}
	categoryNames := map[string]string{
		"drift":    "🎭 角色漂移",
		"timeline": "📜 时间线",
		"threads":  "🧵 剧情伏笔",
	}

	for _, cat := range categories {
		var catIssues []Issue
		for _, i := range r.Issues {
			if i.Category == cat {
				catIssues = append(catIssues, i)
			}
		}
		if len(catIssues) == 0 {
			continue
		}

		fmt.Println()
		fmt.Printf("%s:\n", categoryNames[cat])
		fmt.Println(strings.Repeat("─", 40))
		for _, i := range catIssues {
			icon := severityIcon(i.Severity)
			fmt.Printf("  %s [%s] %s\n", icon, i.Severity, i.Message)
			if len(i.RelevantEvents) > 0 {
				fmt.Printf("      相关事件: %s\n", strings.Join(i.RelevantEvents, ", "))
			}
		}
	}
	fmt.Println()
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityError:
		return "❌"
	case SeverityWarning:
		return "⚠️"
	case SeverityInfo:
		return "ℹ️"
	default:
		return "•"
	}
}

// RunAll runs all consistency checks and returns a combined report.
func RunAll(protagonist *models.Character, world *models.WorldState, events []models.Event) *Report {
	report := &Report{}

	report.Issues = append(report.Issues, CheckCharacterDrift(protagonist, events)...)
	report.Issues = append(report.Issues, CheckTimeline(protagonist.Name, events)...)
	report.Issues = append(report.Issues, CheckThreads(world, events)...)

	return report
}
