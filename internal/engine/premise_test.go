package engine

import (
	"context"
	"testing"

	"github.com/chun/fiction_factory/internal/llm"
)

const premiseYAML = `title: 城郊来电
genre: 都市悬疑
premise: |
  外卖骑手陈野在一次深夜配送中接到一通来自五年前失踪妻子的电话，
  电话那头的声音说："别回家。" 为了查清真相，他重新卷入自己拼命逃离的旧案。
protagonist_name: 陈野
initial_beliefs:
  - "妻子的失踪是一场意外"
  - "只要埋头跑单就能忘记过去"
initial_goals:
  - "查明电话的来源"
  - "过回平静的生活"
initial_fears:
  - "真相会证明自己当年的懦弱"
initial_values:
  - "诚实"
  - "自保"
`

func TestParsePremise(t *testing.T) {
	p, err := parsePremise(premiseYAML)
	if err != nil {
		t.Fatalf("parsePremise failed: %v", err)
	}
	if p.Title != "城郊来电" {
		t.Errorf("title: got %q", p.Title)
	}
	if p.Genre != "都市悬疑" {
		t.Errorf("genre: got %q", p.Genre)
	}
	if p.ProtagonistName != "陈野" {
		t.Errorf("protagonist: got %q", p.ProtagonistName)
	}
	if len(p.InitialBeliefs) != 2 || len(p.InitialValues) != 2 {
		t.Errorf("unexpected initial lists: %+v", p)
	}
}

func TestParsePremise_RejectsEmpty(t *testing.T) {
	if _, err := parsePremise("title: 有题目\npremise: 有梗概\nprotagonist_name: \"\""); err == nil {
		t.Error("expected error for empty protagonist_name")
	}
	if _, err := parsePremise("title: \"\"\npremise: x\nprotagonist_name: y"); err == nil {
		t.Error("expected error for empty title")
	}
}

func TestPremiseGenerator_Generate(t *testing.T) {
	mock := llm.NewMockLLM().WithResponses("前言废话\n\n" + premiseYAML)
	gen := NewPremiseGenerator(mock)

	p, err := gen.Generate(context.Background(), PremiseInput{
		TopicTitle: "外卖骑手深夜接单遇险",
		HeatLabel:  "1234567",
		Summary:    "近日多地报道外卖骑手深夜配送安全问题",
		Source:     "weibo",
		URL:        "https://example.com",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if p.Title != "城郊来电" {
		t.Errorf("title: got %q", p.Title)
	}
}
