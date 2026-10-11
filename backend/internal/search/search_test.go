package search

import "testing"

func TestParseTavily(t *testing.T) {
	raw := []byte(`{"results":[
		{"title":"A","url":"https://www.reuters.com/x","content":"snip","published_date":"Wed, 08 Oct 2026 10:00:00 GMT"},
		{"title":"","url":"https://skip.me","content":"no title"},
		{"title":"B","url":"https://b.org/y","content":"snip","published_date":null}]}`)
	got, err := parseTavily(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
	if got[0].Source != "reuters.com" || got[0].PublishedAt == nil || got[0].PublishedAt.Day() != 8 {
		t.Errorf("first result parsed wrong: %+v", got[0])
	}
	if got[1].PublishedAt != nil {
		t.Errorf("null date should stay nil")
	}
}
