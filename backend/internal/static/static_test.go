package static

import (
	"strings"
	"testing"

	"quiet/internal/api"
)

func TestRender(t *testing.T) {
	summary := "Lawmakers passed the bill."
	understood := "AI laws and rules."
	tests := []struct {
		name    string
		page    api.Page
		want    []string
		notWant []string
	}{
		{
			name: "empty page",
			page: api.Page{Date: "2026-10-11", Sections: []api.Section{}},
			want: []string{"Nothing followed yet."},
		},
		{
			name: "stories, null summary, trial",
			page: api.Page{Date: "2026-10-10", Count: 2, ReadMinutes: 1, Sections: []api.Section{
				{TopicID: 2, Name: "AI policy", Trial: true, Understood: &understood, Items: []api.PageRow{
					{Title: "Bill passes", URL: "https://a.com/1", Source: "a.com", Date: "9 Oct", Summary: &summary},
				}},
				{TopicID: 1, Name: "Batteries", Items: []api.PageRow{
					{Title: "Thin <one>", URL: "https://b.com/2", Source: "b.com", Date: "8 Oct"},
				}},
			}},
			want:    []string{"Sat 10 Oct", "2 stories · about 1 min", "Trying", understood, summary, `href="https://a.com/1"`, "Thin &lt;one&gt;", "That's today."},
			notWant: []string{"<one>"},
		},
		{
			name: "followed topic with nothing today",
			page: api.Page{Date: "2026-10-10", Sections: []api.Section{{TopicID: 1, Name: "Batteries", Items: []api.PageRow{}}}},
			want: []string{"0 stories · about 1 min", "Nothing worth your time here today."},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Render(tc.page)
			if err != nil {
				t.Fatal(err)
			}
			html := string(out)
			for _, w := range tc.want {
				if !strings.Contains(html, w) {
					t.Errorf("missing %q", w)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(html, w) {
					t.Errorf("unexpected %q", w)
				}
			}
		})
	}
}
