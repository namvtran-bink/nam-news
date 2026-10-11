// Package static renders the Today page as a single self-contained HTML file,
// for publishing on any static host. It shows the same JSON the API serves.
package static

import (
	"bytes"
	_ "embed"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"quiet/internal/api"
)

//go:embed page.html
var pageHTML string

var tmpl = template.Must(template.New("page").Funcs(template.FuncMap{
	"headerDate": headerDate,
	"stories": func(n int) string {
		if n == 1 {
			return "1 story"
		}
		return strconv.Itoa(n) + " stories"
	},
	"minutes": func(m int) int { return max(m, 1) },
}).Parse(pageHTML))

// Render returns the page as HTML.
func Render(page api.Page) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, page); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write renders the page to dir/index.html, creating dir if needed, and returns the file path.
func Write(dir string, page api.Page) (string, error) {
	html, err := Render(page)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "index.html")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, html, 0o644); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

// headerDate turns "2026-10-10" into "Sat 10 Oct".
func headerDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("Mon 2 Jan")
}
