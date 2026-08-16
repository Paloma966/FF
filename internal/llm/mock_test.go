package llm

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMockStubFor_Premise(t *testing.T) {
	out := mockStubFor("TRENDING TOPIC (灵感素材)\n标题: x")
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("premise stub is not valid YAML: %v", err)
	}
	if m["title"] == nil || m["protagonist_name"] == nil {
		t.Errorf("premise stub missing fields: %v", m)
	}
}

func TestMockStubFor_Director(t *testing.T) {
	out := mockStubFor("TASK\n====\nFollow the seven-stage analysis framework")
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("director stub is not valid YAML: %v", err)
	}
	sel, ok := m["selected_event"].(map[string]interface{})
	if !ok || sel["title"] == nil {
		t.Errorf("director stub missing selected_event.title: %v", m)
	}
}

func TestMockStubFor_Verdicts(t *testing.T) {
	for _, prompt := range []string{
		"THE DIRECTOR'S PROPOSED EVENT (YAML)",
		"THE PROTAGONIST'S REACTION (YAML)",
	} {
		if !strings.Contains(mockStubFor(prompt), "approved: true") {
			t.Errorf("verdict stub wrong for %q: %s", prompt, mockStubFor(prompt))
		}
	}
}

func TestMockStubFor_Reaction(t *testing.T) {
	out := mockStubFor("THE EVENT YOU JUST EXPERIENCED")
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("reaction stub is not valid YAML: %v", err)
	}
	if m["emotional_response"] == nil || m["decision"] == nil {
		t.Errorf("reaction stub missing fields: %v", m)
	}
}

func TestMockStubFor_RevisedEvent(t *testing.T) {
	out := mockStubFor("YOUR ORIGINAL EVENT (YAML)")
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("revised event stub is not valid YAML: %v", err)
	}
	if m["selected_event"] == nil {
		t.Errorf("revised event stub missing selected_event: %v", m)
	}
}

func TestMockStubFor_Chapter(t *testing.T) {
	out := mockStubFor("WRITING TASK\n===")
	if !strings.HasPrefix(out, "# ") {
		t.Errorf("chapter stub should start with a markdown heading, got %q", out[:20])
	}
}

func TestMockStubFor_Fallback(t *testing.T) {
	out := mockStubFor("unknown prompt type")
	if !strings.HasPrefix(out, "[MOCK RESPONSE") {
		t.Errorf("expected fallback echo, got %q", out)
	}
}
