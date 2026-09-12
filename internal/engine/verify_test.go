package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Paloma966/FF/internal/llm"
	"github.com/Paloma966/FF/internal/models"
)

const approvedVerdict = "approved: true\nissues: []\nsuggestions: 通过\n"

const rejectedVerdict = `approved: false
issues:
  - "事件与主角当前信念冲突，缺乏过渡"
  - "未来钩子与事件事实无关"
suggestions: |
  放缓信念变化节奏，钩子应从事件事实中自然生长。
`

func TestParseVerdict(t *testing.T) {
	v, err := parseVerdict("一些前言\n\n" + approvedVerdict)
	if err != nil {
		t.Fatalf("parseVerdict failed: %v", err)
	}
	if !v.Approved {
		t.Error("expected approved=true")
	}

	v, err = parseVerdict(rejectedVerdict)
	if err != nil {
		t.Fatalf("parseVerdict failed: %v", err)
	}
	if v.Approved {
		t.Error("expected approved=false")
	}
	if len(v.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(v.Issues))
	}

	if _, err := parseVerdict("完全没有 approved 字段的文本"); err == nil {
		t.Error("expected error when approved field missing")
	}
}

func testEvent() *models.Event {
	return &models.Event{
		Title:        "阁楼上的信",
		Time:         "第2天，傍晚",
		Participants: []string{"林恩"},
		Description:  "林恩在阁楼里发现了父亲留下的未寄出的信。",
		FactsChanged: []models.FactChange{
			{Before: "父亲的失踪是意外", After: "父亲的失踪另有隐情"},
		},
		BeliefChanges: []models.BeliefChange{
			{Character: "林恩", Before: "believes the world is simple", After: "suspects his father had secrets"},
		},
		DirectorIntent: "推进核心悬念",
		TensionLevel:   6,
		Tone:           "mysterious",
	}
}

func testCharacter() *models.Character {
	return &models.Character{
		Name:    "林恩",
		Beliefs: []string{"believes the world is simple"},
		Goals:   []string{"查明父亲失踪的真相"},
		Fears:   []string{"真相会毁掉家庭"},
		Values:  []string{"诚实", "亲情"},
	}
}

func TestProtagonistAgent_VerifyEvent(t *testing.T) {
	mock := llm.NewMockLLM().WithResponses(approvedVerdict)
	agent := NewProtagonistAgent(mock)

	v, err := agent.VerifyEvent(context.Background(), VerifyEventInput{
		Character: testCharacter(),
		Facts:     []string{"父亲在五年前失踪"},
		Event:     testEvent(),
	})
	if err != nil {
		t.Fatalf("VerifyEvent failed: %v", err)
	}
	if !v.Approved {
		t.Error("expected approved")
	}
}

func TestDirectorAgent_VerifyReaction(t *testing.T) {
	mock := llm.NewMockLLM().WithResponses(rejectedVerdict)
	agent := NewDirectorAgent(mock)

	v, err := agent.VerifyReaction(context.Background(), VerifyReactionInput{
		Event:     testEvent(),
		Character: testCharacter(),
		Facts:     []string{"父亲在五年前失踪"},
		Reaction: &models.ProtagonistReaction{
			EmotionalResponse: "震惊",
			Decision:          "继续追查",
		},
	})
	if err != nil {
		t.Fatalf("VerifyReaction failed: %v", err)
	}
	if v.Approved {
		t.Error("expected rejected")
	}
	if len(v.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(v.Issues))
	}
}

const revisedEventYAML = `selected_event:
  title: "修订后的秘密信件"
  time: "第2天，傍晚"
  participants: ["林恩"]
  description: "林恩在阁楼整理旧物时，从地板夹层里找到了父亲留下的一封信。"
  facts_changed:
    - before: "父亲的失踪是意外"
      after: "父亲的失踪另有隐情"
  belief_changes:
    - character: "林恩"
      before: "believes the world is simple"
      after: "suspects his father had secrets"
  future_hooks:
    - id: "hook-fathers-secret"
      description: "信中暗示父亲掌握着某个秘密，但没有说明内容"
      hints_at: "父亲失踪的真正原因"
      urgency: "soon"
  caused_by: []
  resolves_hooks: []
  director_intent: "推进核心悬念"
  tension_level: 6
  tone: "mysterious"
`

func TestDirectorAgent_ReviseEvent(t *testing.T) {
	mock := llm.NewMockLLM().WithResponses(revisedEventYAML)
	agent := NewDirectorAgent(mock)

	v, err := parseVerdict(rejectedVerdict)
	if err != nil {
		t.Fatal(err)
	}

	revised, err := agent.ReviseEvent(context.Background(), testEvent(), v)
	if err != nil {
		t.Fatalf("ReviseEvent failed: %v", err)
	}
	if revised.Title != "修订后的秘密信件" {
		t.Errorf("expected revised title, got %q", revised.Title)
	}
	if revised.Lifecycle != models.EventProposed {
		t.Errorf("expected lifecycle proposed, got %q", revised.Lifecycle)
	}
	if len(revised.FutureHooks) != 1 {
		t.Errorf("expected 1 future hook, got %d", len(revised.FutureHooks))
	}
}

func TestProtagonistAgent_ReviseReaction(t *testing.T) {
	revised := strings.Replace(validProtagonistYAML(), "I will not confront him yet", "我决定先不声张，暗中调查", 1)
	mock := llm.NewMockLLM().WithResponses(revised)
	agent := NewProtagonistAgent(mock)

	v, err := parseVerdict(rejectedVerdict)
	if err != nil {
		t.Fatal(err)
	}

	reaction, err := agent.ReviseReaction(context.Background(), testEvent(),
		&models.ProtagonistReaction{EmotionalResponse: "旧", Decision: "旧"}, v)
	if err != nil {
		t.Fatalf("ReviseReaction failed: %v", err)
	}
	if !strings.Contains(reaction.Decision, "暗中调查") {
		t.Errorf("expected revised decision, got %q", reaction.Decision)
	}
}

// revisedReactionYAML is a parseable revised protagonist reaction.
func revisedReactionYAML() string {
	return strings.Replace(validProtagonistYAML(),
		"I will not confront him yet. I need more proof.",
		"我决定先不声张，暗中调查，找到他无法抵赖的证据。", 1)
}
