package curate

import (
	"context"
	"errors"
	"testing"

	"quiet/internal/search"
)

func TestValidateSpec(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		wantErr bool
		want    string // expected Name
	}{
		{"plain", `{"name":"AI policy","query":"AI regulation news","include":["laws"],"exclude":[],"understood":"AI laws and rules."}`, false, "AI policy"},
		{"fenced", "```json\n{\"name\":\"Batteries\",\"query\":\"battery news\",\"include\":[],\"exclude\":[],\"understood\":\"Battery news.\"}\n```", false, "Batteries"},
		{"missing query", `{"name":"X","query":"","understood":"x"}`, true, ""},
		{"missing understood", `{"name":"X","query":"x"}`, true, ""},
		{"not json", `Sure! Here you go.`, true, ""},
		{"broken json", `{"name":"X",`, true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateSpec(tc.reply)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && got.Name != tc.want {
				t.Errorf("name = %q, want %q", got.Name, tc.want)
			}
		})
	}
}

func TestValidateKeep(t *testing.T) {
	results := []search.Result{{URL: "https://a.com/1"}, {URL: "https://b.com/2"}, {URL: "https://c.com/3"}, {URL: "https://d.com/4"}, {URL: "https://e.com/5"}}
	tests := []struct {
		name        string
		reply       string
		wantErr     bool
		wantURLs    []string
		wantNullIdx []int // indexes whose summary must be nil
	}{
		{"zero items is valid", `{"keep":[]}`, false, nil, nil},
		{"keeps and nulls", `{"keep":[{"url":"https://a.com/1","summary":"A thing happened.","reason":"matched"},{"url":"https://b.com/2","summary":null,"reason":"matched"},{"url":"https://c.com/3","summary":"  ","reason":"x"}]}`,
			false, []string{"https://a.com/1", "https://b.com/2", "https://c.com/3"}, []int{1, 2}},
		{"drops duplicate urls", `{"keep":[{"url":"https://a.com/1","summary":"x"},{"url":"https://a.com/1","summary":"y"}]}`, false, []string{"https://a.com/1"}, nil},
		{"caps at four", `{"keep":[{"url":"https://a.com/1"},{"url":"https://b.com/2"},{"url":"https://c.com/3"},{"url":"https://d.com/4"},{"url":"https://e.com/5"}]}`,
			false, []string{"https://a.com/1", "https://b.com/2", "https://c.com/3", "https://d.com/4"}, []int{0, 1, 2, 3}},
		{"rejects invented url", `{"keep":[{"url":"https://made.up/x","summary":"x"}]}`, true, nil, nil},
		{"rejects missing keep", `{"items":[]}`, true, nil, nil},
		{"rejects prose", `I kept two items.`, true, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateKeep(tc.reply, results)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if len(got) != len(tc.wantURLs) {
				t.Fatalf("got %d items, want %d", len(got), len(tc.wantURLs))
			}
			for i, u := range tc.wantURLs {
				if got[i].URL != u {
					t.Errorf("item %d url = %q, want %q", i, got[i].URL, u)
				}
			}
			for _, i := range tc.wantNullIdx {
				if got[i].Summary != nil {
					t.Errorf("item %d summary = %q, want nil", i, *got[i].Summary)
				}
			}
		})
	}
}

type scriptedLLM struct {
	replies []string
	calls   int
}

func (s *scriptedLLM) Complete(context.Context, string, string) (string, error) {
	if s.calls >= len(s.replies) {
		return "", errors.New("no more replies")
	}
	s.calls++
	return s.replies[s.calls-1], nil
}

func TestCurateRetry(t *testing.T) {
	results := []search.Result{{URL: "https://a.com/1", Title: "A"}}
	good := `{"keep":[{"url":"https://a.com/1","summary":"A happened.","reason":"matched"}]}`
	tests := []struct {
		name      string
		replies   []string
		wantErr   bool
		wantCalls int
	}{
		{"first try", []string{good}, false, 1},
		{"retry once then ok", []string{"not json", good}, false, 2},
		{"two failures give up", []string{"not json", "still not"}, true, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := &scriptedLLM{replies: tc.replies}
			kept, err := Curate(context.Background(), l, Spec{Name: "A"}, results)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if l.calls != tc.wantCalls {
				t.Errorf("calls = %d, want %d", l.calls, tc.wantCalls)
			}
			if err != nil && kept != nil {
				t.Errorf("kept = %v on failure, want nil", kept)
			}
		})
	}
}

func TestCurateNoResultsSkipsModel(t *testing.T) {
	l := &scriptedLLM{}
	kept, err := Curate(context.Background(), l, Spec{Name: "A"}, nil)
	if err != nil || kept != nil || l.calls != 0 {
		t.Fatalf("kept=%v err=%v calls=%d", kept, err, l.calls)
	}
}
