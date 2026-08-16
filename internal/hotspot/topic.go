// Package hotspot crawls trending topics from Chinese web platforms
// (Weibo, Zhihu, Baidu) and provides fallback selection logic.
package hotspot

import (
	"context"
	"sort"
)

// Topic is a single trending topic from a hotspot source.
type Topic struct {
	Title     string // 热点标题
	Heat      int64  // 热度数值（归一化为可比较的整数）
	HeatLabel string // 原始热度展示文本，如 "1187 万热度"
	Summary   string // 摘要 / 备注
	URL       string // 可跳转链接
	Source    string // "weibo" | "zhihu" | "baidu"
}

// Source is a single hotspot provider.
type Source interface {
	Name() string
	Fetch(ctx context.Context) ([]Topic, error)
}

// SortByHeatDesc sorts topics by heat in descending order (in place).
func SortByHeatDesc(topics []Topic) {
	sort.SliceStable(topics, func(i, j int) bool {
		return topics[i].Heat > topics[j].Heat
	})
}
