package hotspot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Baidu fetches the Baidu hot search board.
type Baidu struct {
	baseURL string
	client  *http.Client
}

// NewBaidu creates a Baidu source using the default endpoint.
func NewBaidu() *Baidu {
	return newBaiduWithBaseURL("https://top.baidu.com/api/board?platform=wise&tab=realtime")
}

func newBaiduWithBaseURL(baseURL string) *Baidu {
	return &Baidu{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Name returns the source identifier.
func (b *Baidu) Name() string { return "baidu" }

type baiduResponse struct {
	Data struct {
		Cards []struct {
			Content []struct {
				Word     string `json:"word"`
				HotScore string `json:"hotScore"`
				Desc     string `json:"desc"`
				URL      string `json:"url"`
			} `json:"content"`
		} `json:"cards"`
	} `json:"data"`
}

// Fetch retrieves the current Baidu hot search topics.
func (b *Baidu) Fetch(ctx context.Context) ([]Topic, error) {
	var resp baiduResponse
	if err := getJSON(ctx, b.client, b.baseURL, "https://top.baidu.com/board?tab=realtime", &resp); err != nil {
		return nil, fmt.Errorf("baidu fetch: %w", err)
	}

	var topics []Topic
	for _, card := range resp.Data.Cards {
		for _, item := range card.Content {
			if item.Word == "" {
				continue
			}
			heat, _ := strconv.ParseInt(item.HotScore, 10, 64)
			link := item.URL
			if link == "" {
				link = "https://www.baidu.com/s?wd=" + url.QueryEscape(item.Word)
			}
			topics = append(topics, Topic{
				Title:     item.Word,
				Heat:      heat,
				HeatLabel: item.HotScore,
				Summary:   item.Desc,
				URL:       link,
				Source:    b.Name(),
			})
		}
	}
	return topics, nil
}
