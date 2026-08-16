package hotspot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// userAgent is sent with every request to avoid bot filtering.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Crawler fetches topics from multiple sources with automatic fallback.
type Crawler struct {
	sources []Source
}

// NewCrawler creates a crawler with the default source order:
// weibo → zhihu → baidu.
func NewCrawler() *Crawler {
	return NewCrawlerWithSources([]Source{
		NewWeibo(),
		NewZhihu(),
		NewBaidu(),
	})
}

// NewCrawlerWithSources creates a crawler with explicit sources (used in tests).
func NewCrawlerWithSources(sources []Source) *Crawler {
	return &Crawler{sources: sources}
}

// Fetch tries each source in order and returns the first non-empty result.
// It returns the topic list and the name of the source that produced it.
// If every source fails, the error aggregates all failures.
func (c *Crawler) Fetch(ctx context.Context) ([]Topic, string, error) {
	var errs []string
	for _, src := range c.sources {
		topics, err := src.Fetch(ctx)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", src.Name(), err))
			continue
		}
		if len(topics) == 0 {
			errs = append(errs, fmt.Sprintf("%s: empty result", src.Name()))
			continue
		}
		return topics, src.Name(), nil
	}
	return nil, "", fmt.Errorf("all hotspot sources failed:\n  - %s", strings.Join(errs, "\n  - "))
}

// Pick selects one topic. With a non-empty keyword it returns the
// highest-heat topic whose title contains the keyword (case-insensitive);
// with an empty keyword it returns the highest-heat topic overall.
func (c *Crawler) Pick(topics []Topic, keyword string) (Topic, error) {
	if len(topics) == 0 {
		return Topic{}, errors.New("no topics available")
	}
	if keyword == "" {
		best := topics[0]
		for _, t := range topics[1:] {
			if t.Heat > best.Heat {
				best = t
			}
		}
		return best, nil
	}

	var matches []Topic
	kw := strings.ToLower(keyword)
	for _, t := range topics {
		if strings.Contains(strings.ToLower(t.Title), kw) {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		return Topic{}, fmt.Errorf("no topic matches %q — use --list-topics to see available titles", keyword)
	}
	best := matches[0]
	for _, t := range matches[1:] {
		if t.Heat > best.Heat {
			best = t
		}
	}
	return best, nil
}

// getJSON performs a GET request with browser-like headers and decodes the
// response body as JSON into target.
func getJSON(ctx context.Context, client *http.Client, url string, referer string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}
