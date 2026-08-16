package hotspot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Zhihu fetches the Zhihu hot list.
type Zhihu struct {
	baseURL string
	client  *http.Client
}

// NewZhihu creates a Zhihu source using the default endpoint.
func NewZhihu() *Zhihu {
	return newZhihuWithBaseURL("https://www.zhihu.com/api/v3/feed/topstory/hot-lists/total?limit=50")
}

func newZhihuWithBaseURL(baseURL string) *Zhihu {
	return &Zhihu{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Name returns the source identifier.
func (z *Zhihu) Name() string { return "zhihu" }

type zhihuResponse struct {
	Data []struct {
		DetailText string `json:"detail_text"`
		Target     struct {
			Title   string `json:"title"`
			Excerpt string `json:"excerpt"`
			URL     string `json:"url"`
		} `json:"target"`
	} `json:"data"`
}

var zhihuHeatNum = regexp.MustCompile(`(\d+(?:\.\d+)?)`)

// parseZhihuHeat converts a label like "1187 万热度" into an integer.
func parseZhihuHeat(label string) int64 {
	m := zhihuHeatNum.FindStringSubmatch(label)
	if m == nil {
		return 0
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	if strings.Contains(label, "万") {
		val *= 10000
	}
	return int64(val)
}

// Fetch retrieves the current Zhihu hot list topics.
func (z *Zhihu) Fetch(ctx context.Context) ([]Topic, error) {
	var resp zhihuResponse
	if err := getJSON(ctx, z.client, z.baseURL, "https://www.zhihu.com/", &resp); err != nil {
		return nil, fmt.Errorf("zhihu fetch: %w", err)
	}

	var topics []Topic
	for _, item := range resp.Data {
		title := item.Target.Title
		if title == "" {
			continue
		}
		link := item.Target.URL
		if link == "" {
			link = "https://www.zhihu.com/search?q=" + url.QueryEscape(title)
		}
		topics = append(topics, Topic{
			Title:     title,
			Heat:      parseZhihuHeat(item.DetailText),
			HeatLabel: item.DetailText,
			Summary:   item.Target.Excerpt,
			URL:       link,
			Source:    z.Name(),
		})
	}
	return topics, nil
}
