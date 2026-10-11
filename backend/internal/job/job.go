// Package job runs topics: search, curate, save. The daily job and the bar both use RunTopic.
package job

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"quiet/internal/curate"
	"quiet/internal/db"
	"quiet/internal/llm"
	"quiet/internal/search"
)

// DedupeWindow is how long a saved URL blocks the same URL for the same topic.
const DedupeWindow = 7 * 24 * time.Hour

type Runner struct {
	DB       *db.DB
	Searcher search.Searcher
	LLM      llm.Client
	Loc      *time.Location
	Clock    func() time.Time // nil means time.Now

	dailyMu sync.Mutex
}

func (r *Runner) Now() time.Time {
	if r.Clock != nil {
		return r.Clock()
	}
	return time.Now()
}

// Today is the current date in the app's time zone, as YYYY-MM-DD.
func (r *Runner) Today() string { return r.Now().In(r.Loc).Format("2006-01-02") }

// StartOfToday is midnight today in the app's time zone.
func (r *Runner) StartOfToday() time.Time {
	n := r.Now().In(r.Loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, r.Loc)
}

// Saved is what one run kept.
type Saved struct {
	Results int
	Items   []db.Item
}

// RunTopic searches, curates and saves one topic. Every call is recorded in runs.
func (r *Runner) RunTopic(ctx context.Context, t db.Topic) (Saved, error) {
	started := r.Now()
	saved, err := r.runTopic(ctx, t)
	run := db.Run{TopicID: t.ID, StartedAt: started, OK: err == nil, NResults: saved.Results, NKept: len(saved.Items)}
	if err != nil {
		run.Error = err.Error()
		log.Printf("topic %d (%s): %v", t.ID, t.Name, err)
	}
	if rerr := r.DB.RecordRun(context.WithoutCancel(ctx), run); rerr != nil {
		log.Printf("record run: %v", rerr)
	}
	return saved, err
}

func (r *Runner) runTopic(ctx context.Context, t db.Topic) (Saved, error) {
	var saved Saved
	results, err := r.Searcher.Search(ctx, t.Query)
	if err != nil {
		return saved, fmt.Errorf("search: %w", err)
	}
	saved.Results = len(results)

	recent, err := r.DB.RecentURLs(ctx, t.ID, r.Now().Add(-DedupeWindow))
	if err != nil {
		return saved, err
	}
	fresh := FilterSeen(results, recent)

	spec := curate.Spec{Name: t.Name, Query: t.Query, Include: t.Include, Exclude: t.Exclude, Understood: t.Understood}
	kept, err := curate.Curate(ctx, r.LLM, spec, fresh)
	if err != nil {
		return saved, fmt.Errorf("curate: %w", err)
	}

	byURL := make(map[string]search.Result, len(fresh))
	for _, res := range fresh {
		byURL[res.URL] = res
	}
	now := r.Now()
	for _, k := range kept {
		res := byURL[k.URL]
		it := db.Item{
			TopicID: t.ID, URL: k.URL, Title: res.Title, Source: res.Source, PublishedAt: res.PublishedAt,
			Summary: k.Summary, Reason: k.Reason, RunDate: r.Today(), CreatedAt: now,
		}
		if err := r.DB.UpsertItem(ctx, it); err != nil {
			return saved, fmt.Errorf("save: %w", err)
		}
		saved.Items = append(saved.Items, it)
	}
	if err := r.DB.TouchLastRun(ctx, t.ID, now); err != nil {
		return saved, err
	}
	return saved, nil
}

// FilterSeen drops results whose URL is in seen, and repeated URLs within results.
func FilterSeen(results []search.Result, seen map[string]bool) []search.Result {
	out := make([]search.Result, 0, len(results))
	dup := map[string]bool{}
	for _, res := range results {
		if seen[res.URL] || dup[res.URL] {
			continue
		}
		dup[res.URL] = true
		out = append(out, res)
	}
	return out
}

// DailyReport summarizes one daily run.
type DailyReport struct {
	Topics int      `json:"topics"`
	OK     int      `json:"ok"`
	Kept   int      `json:"kept"`
	Failed []string `json:"failed"`
}

// ErrDailyRunning is returned when a daily run is already in progress.
var ErrDailyRunning = fmt.Errorf("daily run already in progress")

// RunDaily runs every followed topic in turn. One failure does not stop the rest.
func (r *Runner) RunDaily(ctx context.Context) (DailyReport, error) {
	rep := DailyReport{Failed: []string{}}
	if !r.dailyMu.TryLock() {
		return rep, ErrDailyRunning
	}
	defer r.dailyMu.Unlock()
	topics, err := r.DB.TopicsByStatus(ctx, db.StatusFollowing)
	if err != nil {
		return rep, err
	}
	rep.Topics = len(topics)
	for _, t := range topics {
		if ctx.Err() != nil {
			return rep, ctx.Err()
		}
		tctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		saved, err := r.RunTopic(tctx, t)
		cancel()
		if err != nil {
			rep.Failed = append(rep.Failed, t.Name)
			continue
		}
		rep.OK++
		rep.Kept += len(saved.Items)
	}
	log.Printf("daily: %d topics, %d ok, %d items kept, failed: %v", rep.Topics, rep.OK, rep.Kept, rep.Failed)
	return rep, nil
}
