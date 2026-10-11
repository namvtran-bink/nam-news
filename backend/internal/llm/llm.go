// Package llm sends one prompt to a model and returns its text reply.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Anthropic calls the Messages API (POST /v1/messages).
type Anthropic struct {
	APIKey    string
	Model     string
	BaseURL   string // default https://api.anthropic.com
	MaxTokens int    // default 1500
	HTTP      *http.Client
}

func (a *Anthropic) Complete(ctx context.Context, system, user string) (string, error) {
	base := a.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	maxTokens := a.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1500
	}
	body, _ := json.Marshal(map[string]any{
		"model":      a.Model,
		"max_tokens": maxTokens,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := string(raw)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", fmt.Errorf("anthropic: status %d: %s", resp.StatusCode, msg)
	}
	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("anthropic: decode: %w", err)
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if c.Type == "text" {
			sb.WriteString(c.Text)
		}
	}
	return sb.String(), nil
}
