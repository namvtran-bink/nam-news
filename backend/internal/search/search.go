// Package search finds recent news for a query. Providers sit behind Searcher.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Result struct {
	Title       string
	URL         string
	Source      string // host name, e.g. "reuters.com"
	PublishedAt *time.Time
	Snippet     string
}

type Searcher interface {
	Search(ctx context.Context, query string) ([]Result, error)
}

// Tavily calls the Tavily search API (https://docs.tavily.com) in news mode.
type Tavily struct {
	APIKey     string
	MaxResults int // default 10
	Days       int // look-back window, default 2
	HTTP       *http.Client
	Now        func() time.Time
}

func (t *Tavily) Search(ctx context.Context, query string) ([]Result, error) {
	maxResults, days := t.MaxResults, t.Days
	if maxResults == 0 {
		maxResults = 10
	}
	if days == 0 {
		days = 2
	}
	now := time.Now
	if t.Now != nil {
		now = t.Now
	}
	body, _ := json.Marshal(map[string]any{
		"query":        query,
		"topic":        "news",
		"start_date":   now().AddDate(0, 0, -days).Format("2006-01-02"),
		"max_results":  maxResults,
		"search_depth": "basic",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := t.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("tavily: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily: status %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return parseTavily(raw)
}

func parseTavily(raw []byte) ([]Result, error) {
	var out struct {
		Results []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			Content       string `json:"content"`
			PublishedDate string `json:"published_date"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("tavily: decode: %w", err)
	}
	results := make([]Result, 0, len(out.Results))
	for _, r := range out.Results {
		if r.URL == "" || r.Title == "" {
			continue
		}
		results = append(results, Result{
			Title:       strings.TrimSpace(r.Title),
			URL:         r.URL,
			Source:      SourceFromURL(r.URL),
			PublishedAt: ParseDate(r.PublishedDate),
			Snippet:     strings.TrimSpace(r.Content),
		})
	}
	return results, nil
}

// SourceFromURL returns the host without a leading "www.".
func SourceFromURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(p.Hostname(), "www.")
}

var dateLayouts = []string{
	time.RFC1123, time.RFC1123Z, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
}

// ParseDate accepts the date formats news providers commonly return; nil if none match.
func ParseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
