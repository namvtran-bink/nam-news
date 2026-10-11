// Package curate holds the two prompts (sentence → topic, results → kept items)
// and the strict validation of what the model returns.
package curate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"quiet/internal/llm"
	"quiet/internal/search"
)

// MaxItems is the most items kept per topic per run.
const MaxItems = 4

// Spec is a topic as the owner described it, turned into search and filter rules.
type Spec struct {
	Name       string   `json:"name"`
	Query      string   `json:"query"`
	Include    []string `json:"include"`
	Exclude    []string `json:"exclude"`
	Understood string   `json:"understood"`
}

// Kept is one result the curator chose to keep.
type Kept struct {
	URL     string  `json:"url"`
	Summary *string `json:"summary"`
	Reason  string  `json:"reason"`
}

const topicSystem = `You turn a person's description of a news interest into search and filter rules.
Reply with JSON only, no prose, no code fences, in exactly this shape:
{"name":"...","query":"...","include":["..."],"exclude":["..."],"understood":"..."}
- name: 1-4 words, title case, the topic itself.
- query: a short news search query (under 10 words).
- include: 0-5 short angles the person wants.
- exclude: 0-5 short angles the person does not want.
- understood: one short sentence (under 15 words) restating what you will look for, so the person can check you understood.`

// ParseTopic runs Prompt 1. When current is non-nil, text is an instruction to
// adjust the existing rules (for example "less company news").
func ParseTopic(ctx context.Context, c llm.Client, text string, current *Spec) (Spec, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Spec{}, errors.New("empty topic text")
	}
	user := "Description: " + text
	if current != nil {
		cur, _ := json.Marshal(current)
		user = "Current rules: " + string(cur) + "\nAdjust them according to this instruction, keeping everything else: " + text
	}
	var spec Spec
	err := withRetry(ctx, c, topicSystem, user, func(reply string) error {
		var err error
		spec, err = ValidateSpec(reply)
		return err
	})
	return spec, err
}

// ValidateSpec checks a Prompt 1 reply.
func ValidateSpec(reply string) (Spec, error) {
	var s Spec
	if err := decodeStrict(reply, &s); err != nil {
		return s, err
	}
	s.Name = strings.TrimSpace(s.Name)
	s.Query = strings.TrimSpace(s.Query)
	s.Understood = strings.TrimSpace(s.Understood)
	s.Include = cleanList(s.Include)
	s.Exclude = cleanList(s.Exclude)
	switch {
	case s.Name == "":
		return s, errors.New("missing name")
	case s.Query == "":
		return s, errors.New("missing query")
	case s.Understood == "":
		return s, errors.New("missing understood")
	}
	return s, nil
}

const curateSystem = `You pick news worth a calm reader's time for one topic and summarize it.
Rules:
- Use ONLY the supplied snippets. No outside knowledge. Never fill gaps.
- Keep items that clearly match the include angles; drop anything matching the exclude angles, opinion pieces, reaction threads, listicles, press-release fluff, and duplicates of the same story (keep the best one).
- summary: 1-2 sentences, at most about 45 words, neutral, in your own words (never copy sentences from the snippet).
- If a snippet is too thin to summarize honestly, set "summary": null.
- Prefer fewer, better items. Keep at most 4. Zero items is a valid answer.
- url must be copied exactly from a supplied result.
Reply with JSON only, no prose, no code fences, in exactly this shape:
{"keep":[{"url":"...","summary":"... or null","reason":"matched: ..."}]}`

// Curate runs Prompt 2 over search results and returns the validated keeps.
func Curate(ctx context.Context, c llm.Client, spec Spec, results []search.Result) ([]Kept, error) {
	if len(results) == 0 {
		return nil, nil
	}
	user := curateInput(spec, results)
	var kept []Kept
	err := withRetry(ctx, c, curateSystem, user, func(reply string) error {
		var err error
		kept, err = ValidateKeep(reply, results)
		return err
	})
	return kept, err
}

func curateInput(spec Spec, results []search.Result) string {
	type in struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Source  string `json:"source"`
		Date    string `json:"date,omitempty"`
		Snippet string `json:"snippet"`
	}
	rs := make([]in, len(results))
	for i, r := range results {
		rs[i] = in{Title: r.Title, URL: r.URL, Source: r.Source, Snippet: r.Snippet}
		if r.PublishedAt != nil {
			rs[i].Date = r.PublishedAt.Format("2006-01-02")
		}
	}
	rules, _ := json.Marshal(map[string]any{"topic": spec.Name, "include": nonNil(spec.Include), "exclude": nonNil(spec.Exclude)})
	res, _ := json.Marshal(rs)
	return "Topic rules: " + string(rules) + "\nResults: " + string(res)
}

// ValidateKeep checks a Prompt 2 reply: every URL must be one we supplied,
// duplicates are dropped, empty summaries become null, and at most MaxItems survive.
func ValidateKeep(reply string, results []search.Result) ([]Kept, error) {
	var out struct {
		Keep *[]Kept `json:"keep"`
	}
	if err := decodeStrict(reply, &out); err != nil {
		return nil, err
	}
	if out.Keep == nil {
		return nil, errors.New("missing keep")
	}
	supplied := make(map[string]bool, len(results))
	for _, r := range results {
		supplied[r.URL] = true
	}
	seen := map[string]bool{}
	var kept []Kept
	for _, k := range *out.Keep {
		k.URL = strings.TrimSpace(k.URL)
		if !supplied[k.URL] {
			return nil, fmt.Errorf("unknown url %q", k.URL)
		}
		if seen[k.URL] {
			continue
		}
		seen[k.URL] = true
		if k.Summary != nil {
			s := strings.TrimSpace(*k.Summary)
			if s == "" || strings.EqualFold(s, "null") {
				k.Summary = nil
			} else {
				k.Summary = &s
			}
		}
		kept = append(kept, k)
		if len(kept) == MaxItems {
			break
		}
	}
	return kept, nil
}

// withRetry calls the model, validates the reply, and retries once on failure.
func withRetry(ctx context.Context, c llm.Client, system, user string, validate func(string) error) error {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		reply, err := c.Complete(ctx, system, user)
		if err == nil {
			err = validate(reply)
			if err != nil {
				err = fmt.Errorf("invalid reply: %w", err)
			}
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
		log.Printf("curate: attempt %d failed: %v", attempt, err)
	}
	return lastErr
}

// decodeStrict pulls the JSON object out of a reply (tolerating code fences or
// stray prose around it) and decodes it.
func decodeStrict(reply string, v any) error {
	start := strings.Index(reply, "{")
	end := strings.LastIndex(reply, "}")
	if start < 0 || end < start {
		return errors.New("no JSON object")
	}
	if err := json.Unmarshal([]byte(reply[start:end+1]), v); err != nil {
		return fmt.Errorf("bad JSON: %w", err)
	}
	return nil
}

func cleanList(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
