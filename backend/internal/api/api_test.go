package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quiet/internal/db"
	"quiet/internal/job"
	"quiet/internal/search"
)

type fakeSearcher struct{}

func (fakeSearcher) Search(context.Context, string) ([]search.Result, error) {
	return []search.Result{{URL: "https://a.com/1", Title: "A one", Source: "a.com"}}, nil
}

// fakeLLM answers Prompt 1 or Prompt 2 depending on which system prompt it gets.
type fakeLLM struct{}

func (fakeLLM) Complete(_ context.Context, system, _ string) (string, error) {
	if strings.Contains(system, "search and filter rules") {
		return `{"name":"Batteries","query":"battery news","include":["research"],"exclude":[],"understood":"Battery research."}`, nil
	}
	return `{"keep":[{"url":"https://a.com/1","summary":null,"reason":"matched"}]}`, nil
}

func newServer(t *testing.T, trialCap int) *Server {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	loc, _ := time.LoadLocation("Asia/Saigon")
	now := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	r := &job.Runner{DB: store, Searcher: fakeSearcher{}, LLM: fakeLLM{}, Loc: loc, Clock: func() time.Time { return now }}
	return &Server{DB: store, Runner: r, Token: "secret-token-123456", TrialCapPerDay: trialCap}
}

func do(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuth(t *testing.T) {
	s := newServer(t, 10)
	h := s.Handler()
	tests := []struct {
		token string
		want  int
	}{{"", 401}, {"wrong", 401}, {s.Token, 200}}
	for _, tc := range tests {
		if got := do(t, h, "GET", "/api/today", "", tc.token).Code; got != tc.want {
			t.Errorf("token %q: status %d, want %d", tc.token, got, tc.want)
		}
	}
}

func TestTrialFlowAndCap(t *testing.T) {
	s := newServer(t, 2)
	h := s.Handler()
	tok := s.Token

	rec := do(t, h, "POST", "/api/topics", `{"text":"battery research please"}`, tok)
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var sec Section
	_ = json.Unmarshal(rec.Body.Bytes(), &sec)
	if !sec.Trial || sec.Understood == nil || len(sec.Items) != 1 || sec.Items[0].Summary != nil {
		t.Fatalf("unexpected section: %+v", sec)
	}

	if rec := do(t, h, "POST", "/api/topics", `{"text":"second"}`, tok); rec.Code != 200 {
		t.Fatalf("second create: %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/topics", `{"text":"third"}`, tok); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over cap: %d, want 429", rec.Code)
	}

	// Follow the first; it moves after the remaining trial.
	if rec := do(t, h, "POST", "/api/topics/1/follow", "", tok); rec.Code != 200 {
		t.Fatalf("follow: %d", rec.Code)
	}
	var page Page
	_ = json.Unmarshal(do(t, h, "GET", "/api/today", "", tok).Body.Bytes(), &page)
	if len(page.Sections) != 2 || page.Sections[0].TopicID != 2 || !page.Sections[0].Trial || page.Sections[1].Trial {
		t.Fatalf("page order wrong: %+v", page.Sections)
	}
	if page.Count != 2 || page.ReadMinutes != 1 || page.Date != "2026-10-10" {
		t.Fatalf("page totals wrong: %+v", page)
	}
	if page.Sections[1].Understood != nil {
		t.Errorf("followed section should not carry understood")
	}

	if rec := do(t, h, "DELETE", "/api/topics/2", "", tok); rec.Code != 200 {
		t.Fatalf("drop: %d", rec.Code)
	}
	_ = json.Unmarshal(do(t, h, "GET", "/api/today", "", tok).Body.Bytes(), &page)
	if len(page.Sections) != 1 {
		t.Fatalf("dropped topic still shown: %+v", page.Sections)
	}
	if rec := do(t, h, "POST", "/api/topics/2/follow", "", tok); rec.Code != 404 {
		t.Fatalf("follow dropped: %d, want 404", rec.Code)
	}
}
