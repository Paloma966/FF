package hotspot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

const weiboJSON = `{
  "ok": 1,
  "data": {
    "realtime": [
      {"word": "话题甲", "word_scheme": "#话题甲#", "num": 3000000, "note": "第一个话题"},
      {"word": "话题乙", "num": 1200000, "note": ""}
    ]
  }
}`

func TestWeiboFetch(t *testing.T) {
	srv := testServer(t, 200, weiboJSON)
	topics, err := newWeiboWithBaseURL(srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics))
	}
	if topics[0].Title != "话题甲" || topics[0].Heat != 3000000 || topics[0].Source != "weibo" {
		t.Errorf("unexpected first topic: %+v", topics[0])
	}
	if topics[1].Summary != "" {
		t.Errorf("expected empty summary, got %q", topics[1].Summary)
	}
}

func TestWeiboFetchHTTPError(t *testing.T) {
	srv := testServer(t, 500, "boom")
	_, err := newWeiboWithBaseURL(srv.URL).Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

const zhihuJSON = `{
  "data": [
    {"detail_text": "1187 万热度", "target": {"title": "问题甲", "excerpt": "摘要甲", "url": "https://api.zhihu.com/questions/1"}},
    {"detail_text": "9999 热度", "target": {"title": "问题乙", "excerpt": ""}}
  ]
}`

func TestZhihuFetch(t *testing.T) {
	srv := testServer(t, 200, zhihuJSON)
	topics, err := newZhihuWithBaseURL(srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics))
	}
	if topics[0].Heat != 11870000 {
		t.Errorf("expected heat 11870000, got %d", topics[0].Heat)
	}
	if topics[0].URL != "https://api.zhihu.com/questions/1" {
		t.Errorf("unexpected URL: %s", topics[0].URL)
	}
	if topics[1].Heat != 9999 {
		t.Errorf("expected heat 9999, got %d", topics[1].Heat)
	}
	if topics[1].URL == "" {
		t.Error("expected fallback search URL for topic without url")
	}
}

func TestParseZhihuHeat(t *testing.T) {
	cases := map[string]int64{
		"1187 万热度": 11870000,
		"9999 热度":  9999,
		"热度":       0,
		"1.5 万热度":  15000,
	}
	for in, want := range cases {
		if got := parseZhihuHeat(in); got != want {
			t.Errorf("parseZhihuHeat(%q) = %d, want %d", in, got, want)
		}
	}
}

const baiduJSON = `{
  "data": {
    "cards": [
      {"content": [
        {"word": "热词甲", "hotScore": "6543210", "desc": "描述甲", "url": "https://www.baidu.com/s?wd=x"},
        {"word": "热词乙", "hotScore": "123", "desc": ""}
      ]},
      {"content": [
        {"word": "热词丙", "hotScore": "notanumber", "desc": "描述丙"}
      ]}
    ]
  }
}`

func TestBaiduFetch(t *testing.T) {
	srv := testServer(t, 200, baiduJSON)
	topics, err := newBaiduWithBaseURL(srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if len(topics) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(topics))
	}
	if topics[0].Heat != 6543210 || topics[0].Summary != "描述甲" {
		t.Errorf("unexpected first topic: %+v", topics[0])
	}
	if topics[2].Heat != 0 {
		t.Errorf("expected heat 0 for unparsable hotScore, got %d", topics[2].Heat)
	}
	if topics[2].URL == "" {
		t.Error("expected fallback URL for topic without url")
	}
}

type fakeSource struct {
	name    string
	topics  []Topic
	failErr error
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Fetch(ctx context.Context) ([]Topic, error) {
	if f.failErr != nil {
		return nil, f.failErr
	}
	return f.topics, nil
}

func TestCrawlerFallback(t *testing.T) {
	c := NewCrawlerWithSources([]Source{
		&fakeSource{name: "broken", failErr: fmt.Errorf("down")},
		&fakeSource{name: "ok", topics: []Topic{{Title: "A", Heat: 10}}},
	})
	topics, source, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if source != "ok" || len(topics) != 1 {
		t.Errorf("expected fallback to 'ok', got source=%q topics=%d", source, len(topics))
	}
}

func TestCrawlerAllFail(t *testing.T) {
	c := NewCrawlerWithSources([]Source{
		&fakeSource{name: "broken1", failErr: fmt.Errorf("down1")},
		&fakeSource{name: "broken2", failErr: fmt.Errorf("down2")},
	})
	if _, _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("expected error when all sources fail")
	}
}

func TestPick(t *testing.T) {
	c := NewCrawlerWithSources(nil)
	topics := []Topic{
		{Title: "Alpha News", Heat: 100},
		{Title: "beta alpha thing", Heat: 50},
		{Title: "Gamma", Heat: 999},
	}

	got, err := c.Pick(topics, "")
	if err != nil {
		t.Fatalf("Pick failed: %v", err)
	}
	if got.Title != "Gamma" {
		t.Errorf("expected highest-heat 'Gamma', got %q", got.Title)
	}

	got, err = c.Pick(topics, "ALPHA")
	if err != nil {
		t.Fatalf("Pick failed: %v", err)
	}
	if got.Title != "Alpha News" {
		t.Errorf("expected case-insensitive match 'Alpha News', got %q", got.Title)
	}

	if _, err := c.Pick(topics, "nonexistent"); err == nil {
		t.Error("expected error for non-matching keyword")
	}
}

func TestSortByHeatDesc(t *testing.T) {
	topics := []Topic{
		{Title: "low", Heat: 1},
		{Title: "high", Heat: 100},
		{Title: "mid", Heat: 50},
	}
	SortByHeatDesc(topics)
	if topics[0].Title != "high" || topics[1].Title != "mid" || topics[2].Title != "low" {
		t.Errorf("unexpected order: %+v", topics)
	}
}
