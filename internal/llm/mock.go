package llm

import (
	"context"
	"fmt"
	"strings"
)

// MockLLM is a mock LLM client for testing and dry-run demos.
// With WithResponses it returns scripted responses in sequence.
// Without scripts it detects the prompt type and returns a structurally
// valid stub (valid YAML for the agents, prose for the chapter writer),
// so `ff run --provider mock` can exercise the entire pipeline offline.
type MockLLM struct {
	responses []string
	callCount int
}

// NewMockLLM creates a new MockLLM.
func NewMockLLM() *MockLLM {
	return &MockLLM{}
}

// WithResponses configures the mock to return specific responses in sequence.
func (m *MockLLM) WithResponses(responses ...string) *MockLLM {
	m.responses = responses
	return m
}

// Complete returns the next pre-configured response, or a type-aware stub.
func (m *MockLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if m.callCount < len(m.responses) {
		resp := m.responses[m.callCount]
		m.callCount++
		return CompletionResponse{Text: resp}, nil
	}
	m.callCount++
	return CompletionResponse{Text: mockStubFor(req.UserPrompt)}, nil
}

// ProviderName returns "mock".
func (m *MockLLM) ProviderName() string {
	return "mock"
}

// CallCount returns how many times Complete was called.
func (m *MockLLM) CallCount() int {
	return m.callCount
}

// Reset resets the call count.
func (m *MockLLM) Reset() {
	m.callCount = 0
}

// mockStubFor returns a valid stub for the given rendered prompt.
func mockStubFor(prompt string) string {
	switch {
	case strings.Contains(prompt, "TRENDING TOPIC"):
		return mockPremiseYAML
	case strings.Contains(prompt, "seven-stage"):
		return mockDirectorYAML
	case strings.Contains(prompt, "THE DIRECTOR'S PROPOSED EVENT"):
		return mockApprovedVerdict
	case strings.Contains(prompt, "YOUR ORIGINAL EVENT"):
		return mockRevisedEventYAML
	case strings.Contains(prompt, "THE EVENT YOU JUST EXPERIENCED"):
		return mockReactionYAML
	case strings.Contains(prompt, "THE PROTAGONIST'S REACTION"):
		return mockApprovedVerdict
	case strings.Contains(prompt, "YOUR ORIGINAL REACTION"):
		return mockReactionYAML
	case strings.Contains(prompt, "WRITING TASK"):
		return mockChapterText
	default:
		return fmt.Sprintf("[MOCK RESPONSE %d]: received prompt with %d chars", 0, len(prompt))
	}
}

const mockPremiseYAML = `title: 未寄出的信
genre: 都市悬疑
premise: |
  普通青年林晚在一次搬家整理旧物时，发现了一封从未寄出的信。
  信中的只言片语指向一段被刻意掩埋的过去，而寄信人竟是他早已离世的母亲。
protagonist_name: 林晚
initial_beliefs:
  - "相信母亲的离世是一场意外"
  - "相信世界是简单的"
initial_goals:
  - "查明这封信的来历"
  - "找到信中提到的那个人"
initial_fears:
  - "真相会毁掉平静的生活"
initial_values:
  - "诚实"
  - "守护家人"
`

const mockDirectorYAML = `state_digest: |
  故事持续推进，主角刚刚获得了一条关键线索，内心充满疑问。

thread_inventory: []

arc_status: |
  主角正处于"追寻真相"的弧线上，需要新的证据推动下一步行动。

tension_calibration: |
  维持当前节奏，为下一处转折蓄力。

candidates:
  - title: "新的线索"
    description: "主角顺着旧信中的地址找到一处早已废弃的老宅"
    threads_used: []
    beliefs_tested: []
    tension_estimate: 5

evaluation:
  scores:
    - title: "新的线索"
      thread_progression: 3
      character_arc: 3
      pacing_fit: 3
      consistency: 3
      surprise: 3
  selected: "新的线索"
  rationale: "推进主线，保持悬念"

selected_event:
  title: "新的线索"
  time: "第1天，傍晚"
  participants: ["林晚"]
  description: "林晚按照旧信中的地址找到城郊一处废弃的老宅。大门虚掩，屋内积满灰尘，桌上放着一张与母亲合影的旧照片。"
  facts_changed:
    - before: "旧信只提到一个模糊的地址"
      after: "老宅中发现了母亲与陌生人的合影"
  belief_changes:
    - character: "林晚"
      before: "相信母亲的离世是一场意外"
      after: "开始怀疑母亲的离世另有隐情"
  future_hooks:
    - id: "hook-old-photo"
      description: "照片上的陌生人身份不明"
      hints_at: "母亲隐瞒的过去"
      urgency: "soon"
  caused_by: []
  resolves_hooks: []
  director_intent: "推进主线，用新证据加深悬念"
  tension_level: 5
  tone: "mysterious"
`

const mockRevisedEventYAML = `selected_event:
  title: "新的线索"
  time: "第1天，傍晚"
  participants: ["林晚"]
  description: "林晚按照旧信中的地址找到城郊一处废弃的老宅，在落满灰尘的桌上发现一张母亲与陌生人的合影。"
  facts_changed:
    - before: "旧信只提到一个模糊的地址"
      after: "老宅中发现了母亲与陌生人的合影"
  belief_changes:
    - character: "林晚"
      before: "相信母亲的离世是一场意外"
      after: "开始怀疑母亲的离世另有隐情"
  future_hooks:
    - id: "hook-old-photo"
      description: "照片上的陌生人身份不明"
      hints_at: "母亲隐瞒的过去"
      urgency: "soon"
  caused_by: []
  resolves_hooks: []
  director_intent: "推进主线，用新证据加深悬念"
  tension_level: 5
  tone: "mysterious"
`

const mockReactionYAML = `emotional_response: |
  心里涌起一阵复杂的情绪，既紧张又莫名地安心。仿佛离某个真相又近了一步。

decision: |
  我决定查清照片上那个陌生人的身份，从这栋老宅的旧物开始。

new_beliefs: []

modified_beliefs:
  - "相信母亲的离世是一场意外 -> 母亲的离世或许另有隐情"

abandoned_beliefs: []

strengthened_values:
  - "坚持"

challenged_values: []

memory_formed: |
  推开门的那一刻，灰尘在夕光里翻飞，像旧时光活了过来。

internal_monologue: |
  妈妈，你到底还有什么没告诉我？
`

const mockApprovedVerdict = "approved: true\nissues: []\nsuggestions: 通过\n"

const mockChapterText = "# 新的线索\n\n傍晚的风从老宅半掩的门缝里灌进来，带着陈年木头的气味。\n\n林晚站在门槛外，手里攥着那封旧信。信上的地址是母亲的字迹，一笔一划，熟悉得让人心口发紧。\n\n他推开门。灰尘在夕光里翻飞。\n\n桌上那张合影已经泛黄——母亲年轻时的脸，和一个他从未见过的男人。\n\n***\n\n林晚把照片翻过来，背面有一行褪色的字：'别来找我。'\n\n可他已经来了。\n"
