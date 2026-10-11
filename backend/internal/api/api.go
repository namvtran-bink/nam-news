// Package api serves the JSON API the web app (or any other UI) renders.
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"

	"quiet/internal/curate"
	"quiet/internal/db"
	"quiet/internal/job"
)

// MaxPageItems caps the whole page.
const MaxPageItems = 15

type Server struct {
	DB             *db.DB
	Runner         *job.Runner
	Token          string
	TrialCapPerDay int
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/today", s.today)
	mux.HandleFunc("POST /api/topics", s.createTopic)
	mux.HandleFunc("POST /api/topics/{id}/follow", s.follow)
	mux.HandleFunc("DELETE /api/topics/{id}", s.drop)
	mux.HandleFunc("PATCH /api/topics/{id}", s.changeTopic)
	mux.HandleFunc("POST /api/admin/run-daily", s.runDaily)
	return s.auth(mux)
}

func (s *Server) auth(next http.Handler) http.Handler {
	want := []byte("Bearer " + s.Token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if s.Token == "" || subtle.ConstantTimeCompare(got, want) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Page is the whole Today page.
type Page struct {
	Date        string    `json:"date"`
	ReadMinutes int       `json:"readMinutes"`
	Count       int       `json:"count"`
	Sections    []Section `json:"sections"`
}

type Section struct {
	TopicID    int64     `json:"topicId"`
	Name       string    `json:"name"`
	Trial      bool      `json:"trial"`
	Understood *string   `json:"understood"`
	Items      []PageRow `json:"items"`
}

type PageRow struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Source  string  `json:"source"`
	Date    string  `json:"date"`
	Summary *string `json:"summary"`
}

func (s *Server) today(w http.ResponseWriter, r *http.Request) {
	page, err := s.BuildPage(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// BuildPage assembles today's page: today's trial topics first, then followed
// topics in creation order, capped at MaxPageItems items in total.
func (s *Server) BuildPage(ctx context.Context) (Page, error) {
	today := s.Runner.Today()
	page := Page{Date: today, Sections: []Section{}}

	trials, err := s.DB.TopicsByStatus(ctx, db.StatusTrial)
	if err != nil {
		return page, err
	}
	following, err := s.DB.TopicsByStatus(ctx, db.StatusFollowing)
	if err != nil {
		return page, err
	}
	start := s.Runner.StartOfToday()
	var topics []db.Topic
	for _, t := range trials {
		if !t.CreatedAt.Before(start) {
			topics = append(topics, t)
		}
	}
	topics = append(topics, following...)

	for _, t := range topics {
		sec, err := s.section(ctx, t, today)
		if err != nil {
			return page, err
		}
		if room := MaxPageItems - page.Count; len(sec.Items) > room {
			sec.Items = sec.Items[:room]
		}
		page.Count += len(sec.Items)
		page.Sections = append(page.Sections, sec)
	}
	page.ReadMinutes = int(math.Ceil(float64(page.Count) * 0.5))
	return page, nil
}

func (s *Server) section(ctx context.Context, t db.Topic, runDate string) (Section, error) {
	sec := Section{TopicID: t.ID, Name: t.Name, Trial: t.Status == db.StatusTrial, Items: []PageRow{}}
	if sec.Trial && t.Understood != "" {
		u := t.Understood
		sec.Understood = &u
	}
	items, err := s.DB.ItemsForRunDate(ctx, t.ID, runDate)
	if err != nil {
		return sec, err
	}
	for _, it := range items {
		sec.Items = append(sec.Items, PageRow{
			Title: it.Title, URL: it.URL, Source: it.Source, Date: s.displayDate(it), Summary: it.Summary,
		})
	}
	return sec, nil
}

func (s *Server) displayDate(it db.Item) string {
	t := it.CreatedAt
	if it.PublishedAt != nil {
		t = *it.PublishedAt
	}
	return t.In(s.Runner.Loc).Format("2 Jan")
}

type textBody struct {
	Text string `json:"text"`
}

func readText(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body textBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "body must be {\"text\": \"...\"}")
		return "", false
	}
	text := strings.TrimSpace(body.Text)
	if text == "" || len(text) > 300 {
		writeError(w, http.StatusBadRequest, "text must be 1-300 characters")
		return "", false
	}
	return text, true
}

func (s *Server) createTopic(w http.ResponseWriter, r *http.Request) {
	text, ok := readText(w, r)
	if !ok {
		return
	}
	n, err := s.DB.CountTopicsCreatedSince(r.Context(), s.Runner.StartOfToday())
	if err != nil {
		internalError(w, err)
		return
	}
	if n >= s.TrialCapPerDay {
		writeError(w, http.StatusTooManyRequests, "daily limit for new topics reached")
		return
	}
	t, err := CreateTrial(r.Context(), s.Runner, text)
	if err != nil {
		upstreamError(w, err)
		return
	}
	s.writeSection(w, r.Context(), t)
}

// CreateTrial parses free text into a trial topic, saves it and runs it once.
// The topic is kept even if the run fails, so the owner sees it and can retry with Change.
func CreateTrial(ctx context.Context, runner *job.Runner, text string) (db.Topic, error) {
	spec, err := curate.ParseTopic(ctx, runner.LLM, text, nil)
	if err != nil {
		return db.Topic{}, err
	}
	t, err := runner.DB.CreateTopic(ctx, db.Topic{
		Name: spec.Name, Query: spec.Query, Include: spec.Include, Exclude: spec.Exclude,
		Understood: spec.Understood, Status: db.StatusTrial, CreatedAt: runner.Now(),
	})
	if err != nil {
		return t, err
	}
	_, err = runner.RunTopic(ctx, t)
	return t, err
}

func (s *Server) changeTopic(w http.ResponseWriter, r *http.Request) {
	t, ok := s.topicFromPath(w, r)
	if !ok {
		return
	}
	text, ok := readText(w, r)
	if !ok {
		return
	}
	current := curate.Spec{Name: t.Name, Query: t.Query, Include: t.Include, Exclude: t.Exclude, Understood: t.Understood}
	spec, err := curate.ParseTopic(r.Context(), s.Runner.LLM, text, &current)
	if err != nil {
		upstreamError(w, err)
		return
	}
	t.Name, t.Query, t.Include, t.Exclude, t.Understood = spec.Name, spec.Query, spec.Include, spec.Exclude, spec.Understood
	if err := s.DB.UpdateTopicRules(r.Context(), t); err != nil {
		internalError(w, err)
		return
	}
	// Today's items were picked under the old rules; replace them.
	if err := s.DB.DeleteItemsForRunDate(r.Context(), t.ID, s.Runner.Today()); err != nil {
		internalError(w, err)
		return
	}
	if _, err := s.Runner.RunTopic(r.Context(), t); err != nil {
		upstreamError(w, err)
		return
	}
	s.writeSection(w, r.Context(), t)
}

func (s *Server) writeSection(w http.ResponseWriter, ctx context.Context, t db.Topic) {
	sec, err := s.section(ctx, t, s.Runner.Today())
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sec)
}

func (s *Server) follow(w http.ResponseWriter, r *http.Request) {
	s.setStatus(w, r, db.StatusFollowing)
}
func (s *Server) drop(w http.ResponseWriter, r *http.Request) { s.setStatus(w, r, db.StatusDropped) }

func (s *Server) setStatus(w http.ResponseWriter, r *http.Request, status string) {
	t, ok := s.topicFromPath(w, r)
	if !ok {
		return
	}
	if err := s.DB.SetStatus(r.Context(), t.ID, status); err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topicId": t.ID, "status": status})
}

func (s *Server) topicFromPath(w http.ResponseWriter, r *http.Request) (db.Topic, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad topic id")
		return db.Topic{}, false
	}
	t, err := s.DB.GetTopic(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) || (err == nil && t.Status == db.StatusDropped) {
		writeError(w, http.StatusNotFound, "topic not found")
		return t, false
	}
	if err != nil {
		internalError(w, err)
		return t, false
	}
	return t, true
}

func (s *Server) runDaily(w http.ResponseWriter, r *http.Request) {
	rep, err := s.Runner.RunDaily(r.Context())
	if errors.Is(err, job.ErrDailyRunning) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func internalError(w http.ResponseWriter, err error) {
	log.Printf("api: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

// upstreamError covers failures talking to the search or model provider.
func upstreamError(w http.ResponseWriter, err error) {
	log.Printf("api: upstream: %v", err)
	writeError(w, http.StatusBadGateway, "couldn't fetch news for that right now")
}
