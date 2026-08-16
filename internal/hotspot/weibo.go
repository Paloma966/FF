package hotspot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Weibo fetches the Weibo hot search list.
type Weibo struct {
	baseURL string
	client  *http.Client
}

// NewWeibo creates a Weibo source using the default endpoint.
func NewWeibo() *Weibo {
	return newWeiboWithBaseURL("https://weibo.com/ajax/side/hotSearch")
}

func newWeiboWithBaseURL(baseURL string) *Weibo {
	return &Weibo{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Name returns the source identifier.
func (w *Weibo) Name() string { return "weibo" }

type weiboResponse struct {
	OK   int `json:"ok"`
	Data struct {
		Realtime []struct {
			Word       string `json:"word"`
			WordScheme string `json:"word_scheme"`
			Num        int64  `json:"num"`
			Note       string `json:"note"`
		} `json:"realtime"`
	} `json:"data"`
}

// Fetch retrieves the current Weibo hot search topics.
func (w *Weibo) Fetch(ctx context.Context) ([]Topic, error) {
	var resp weiboResponse
	if err := getJSON(ctx, w.client, w.baseURL, "https://weibo.com/", &resp); err != nil {
		return nil, fmt.Errorf("weibo fetch: %w", err)
	}
	if resp.OK != 1 && resp.OK != 0 {
		return nil, fmt.Errorf("weibo fetch: ok=%d", resp.OK)
	}

	var topics []Topic
	for _, item := range resp.Data.Realtime {
		word := item.Word
		if word == "" {
			word = item.WordScheme
		}
		if word == "" {
			continue
		}
		target := item.WordScheme
		if target == "" {
			target = word
		}
		topics = append(topics, Topic{
			Title:     word,
			Heat:      item.Num,
			HeatLabel: fmt.Sprint(item.Num),
			Summary:   item.Note,
			URL:       "https://s.weibo.com/weibo?q=" + url.QueryEscape(target),
			Source:    w.Name(),
		})
	}
	return topics, nil
}
