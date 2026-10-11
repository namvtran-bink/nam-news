package job

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quiet/internal/db"
	"quiet/internal/search"
)

type fakeSearcher struct{ results []search.Result }

func (f *fakeSearcher) Search(context.Context, string) ([]search.Result, error) {
	return f.results, nil
}

// keepAllLLM keeps every result it is shown and records the URLs it saw.
type keepAllLLM struct{ seen [][]string }

func (k *keepAllLLM) Complete(_ context.Context, _, user string) (string, error) {
	i := strings.Index(user, "Results: ")
	var rs []struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal([]byte(user[i+len("Results: "):]), &rs)
	var urls []string
	type keep struct {
		URL     string `json:"url"`
		Summary string `json:"summary"`
		Reason  string `json:"reason"`
	}
	out := struct {
		Keep []keep `json:"keep"`
	}{Keep: []keep{}}
	for _, r := range rs {
		urls = append(urls, r.URL)
		out.Keep = append(out.Keep, keep{URL: r.URL, Summary: "Something happened.", Reason: "matched"})
	}
	k.seen = append(k.seen, urls)
	b, _ := json.Marshal(out)
	return string(b), nil
}

func newRunner(t *testing.T, s search.Searcher, l *keepAllLLM, now *time.Time) *Runner {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	loc, _ := time.LoadLocation("Asia/Saigon")
	return &Runner{DB: store, Searcher: s, LLM: l, Loc: loc, Clock: func() time.Time { return *now }}
}

func results(urls ...string) []search.Result {
	var out []search.Result
	for _, u := range urls {
		out = append(out, search.Result{URL: u, Title: "T " + u, Source: "x.com"})
	}
	return out
}

func TestFilterSeen(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		seen map[string]bool
		want []string
	}{
		{"nothing seen", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"drops seen", []string{"a", "b", "c"}, map[string]bool{"b": true}, []string{"a", "c"}},
		{"drops repeats in results", []string{"a", "a", "b"}, nil, []string{"a", "b"}},
		{"all seen", []string{"a"}, map[string]bool{"a": true}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterSeen(results(tc.in...), tc.seen)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i].URL != tc.want[i] {
					t.Errorf("%d: got %q want %q", i, got[i].URL, tc.want[i])
				}
			}
		})
	}
}

func TestRunTopicDedupeAndCap(t *testing.T) {
	now := time.Date(2026, 10, 10, 5, 30, 0, 0, time.UTC)
	fs := &fakeSearcher{results: results("u1", "u2", "u3", "u4", "u5", "u6")}
	l := &keepAllLLM{}
	r := newRunner(t, fs, l, &now)
	ctx := context.Background()
	topic, err := r.DB.CreateTopic(ctx, db.Topic{Name: "T", Query: "q", Status: db.StatusFollowing, CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		advance   time.Duration
		wantShown []string // URLs the model is shown
		wantSaved int
	}{
		{"first run caps at 4", 0, []string{"u1", "u2", "u3", "u4", "u5", "u6"}, 4},
		{"next day skips saved urls", 24 * time.Hour, []string{"u5", "u6"}, 2},
		{"all seen: model not called", 24 * time.Hour, nil, 0},
		{"after 7 days urls come back", 8 * 24 * time.Hour, []string{"u1", "u2", "u3", "u4", "u5", "u6"}, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now = now.Add(tc.advance)
			calls := len(l.seen)
			saved, err := r.RunTopic(ctx, topic)
			if err != nil {
				t.Fatal(err)
			}
			if len(saved.Items) != tc.wantSaved {
				t.Errorf("saved %d, want %d", len(saved.Items), tc.wantSaved)
			}
			if tc.wantShown == nil {
				if len(l.seen) != calls {
					t.Errorf("model was called with nothing new")
				}
				return
			}
			shown := l.seen[len(l.seen)-1]
			if strings.Join(shown, ",") != strings.Join(tc.wantShown, ",") {
				t.Errorf("shown %v, want %v", shown, tc.wantShown)
			}
			items, _ := r.DB.ItemsForRunDate(ctx, topic.ID, r.Today())
			if len(items) != tc.wantSaved {
				t.Errorf("db has %d items for today, want %d", len(items), tc.wantSaved)
			}
		})
	}
}
