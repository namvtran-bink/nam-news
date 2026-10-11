// Package db owns the SQLite store: migrations and every query the app runs.
package db

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Statuses a topic can have.
const (
	StatusTrial     = "trial"
	StatusFollowing = "following"
	StatusDropped   = "dropped"
)

var ErrNotFound = errors.New("not found")

type Topic struct {
	ID         int64
	Name       string
	Query      string
	Include    []string
	Exclude    []string
	Understood string
	Status     string
	CreatedAt  time.Time
	LastRunAt  *time.Time
}

type Item struct {
	ID          int64
	TopicID     int64
	URL         string
	Title       string
	Source      string
	PublishedAt *time.Time
	Summary     *string
	Reason      string
	RunDate     string // YYYY-MM-DD in the app's time zone
	CreatedAt   time.Time
}

type Run struct {
	TopicID   int64
	StartedAt time.Time
	OK        bool
	Error     string
	NResults  int
	NKept     int
}

type DB struct{ sql *sql.DB }

// Open opens (or creates) the database at path, turns on WAL and applies migrations.
func Open(ctx context.Context, path string) (*DB, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	s, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One writer keeps SQLite simple; network calls never happen inside a query.
	s.SetMaxOpenConns(1)
	d := &DB{sql: s}
	if err := d.migrate(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error { return d.sql.Close() }

func (d *DB) migrate(ctx context.Context) error {
	if _, err := d.sql.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var n int
		if err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE name = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := d.sql.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func toJSON(v []string) string {
	if v == nil {
		v = []string{}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

const topicCols = `id, name, query, include_json, exclude_json, understood, status, created_at, last_run_at`

func scanTopic(row interface{ Scan(...any) error }) (Topic, error) {
	var t Topic
	var inc, exc, created string
	var lastRun sql.NullString
	if err := row.Scan(&t.ID, &t.Name, &t.Query, &inc, &exc, &t.Understood, &t.Status, &created, &lastRun); err != nil {
		return t, err
	}
	_ = json.Unmarshal([]byte(inc), &t.Include)
	_ = json.Unmarshal([]byte(exc), &t.Exclude)
	t.CreatedAt = parseTime(created)
	if lastRun.Valid {
		lr := parseTime(lastRun.String)
		t.LastRunAt = &lr
	}
	return t, nil
}

func (d *DB) CreateTopic(ctx context.Context, t Topic) (Topic, error) {
	res, err := d.sql.ExecContext(ctx,
		`INSERT INTO topics (name, query, include_json, exclude_json, understood, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.Query, toJSON(t.Include), toJSON(t.Exclude), t.Understood, t.Status, fmtTime(t.CreatedAt))
	if err != nil {
		return t, err
	}
	t.ID, err = res.LastInsertId()
	return t, err
}

// UpdateTopicRules replaces a topic's parsed rules (used by PATCH).
func (d *DB) UpdateTopicRules(ctx context.Context, t Topic) error {
	res, err := d.sql.ExecContext(ctx,
		`UPDATE topics SET name = ?, query = ?, include_json = ?, exclude_json = ?, understood = ? WHERE id = ?`,
		t.Name, t.Query, toJSON(t.Include), toJSON(t.Exclude), t.Understood, t.ID)
	return mustAffect(res, err)
}

func (d *DB) SetStatus(ctx context.Context, id int64, status string) error {
	res, err := d.sql.ExecContext(ctx, `UPDATE topics SET status = ? WHERE id = ?`, status, id)
	return mustAffect(res, err)
}

func (d *DB) TouchLastRun(ctx context.Context, id int64, at time.Time) error {
	_, err := d.sql.ExecContext(ctx, `UPDATE topics SET last_run_at = ? WHERE id = ?`, fmtTime(at), id)
	return err
}

func mustAffect(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) GetTopic(ctx context.Context, id int64) (Topic, error) {
	t, err := scanTopic(d.sql.QueryRowContext(ctx, `SELECT `+topicCols+` FROM topics WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// TopicsByStatus returns topics with the given status in creation order.
func (d *DB) TopicsByStatus(ctx context.Context, status string) ([]Topic, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT `+topicCols+` FROM topics WHERE status = ? ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CountTopicsCreatedSince counts topics created via the bar at or after since (any status).
func (d *DB) CountTopicsCreatedSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM topics WHERE created_at >= ?`, fmtTime(since)).Scan(&n)
	return n, err
}

// RecentURLs returns the URLs saved for a topic at or after since.
func (d *DB) RecentURLs(ctx context.Context, topicID int64, since time.Time) (map[string]bool, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT url FROM items WHERE topic_id = ? AND created_at >= ?`, topicID, fmtTime(since))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out[u] = true
	}
	return out, rows.Err()
}

// UpsertItem inserts an item, or refreshes an older copy of the same (topic, url).
func (d *DB) UpsertItem(ctx context.Context, it Item) error {
	var pub any
	if it.PublishedAt != nil {
		pub = fmtTime(*it.PublishedAt)
	}
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO items (topic_id, url, title, source, published_at, summary, reason, run_date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (topic_id, url) DO UPDATE SET
			title = excluded.title, source = excluded.source, published_at = excluded.published_at,
			summary = excluded.summary, reason = excluded.reason, run_date = excluded.run_date,
			created_at = excluded.created_at`,
		it.TopicID, it.URL, it.Title, it.Source, pub, it.Summary, it.Reason, it.RunDate, fmtTime(it.CreatedAt))
	return err
}

// ItemsForRunDate returns a topic's items for one run date, newest first.
func (d *DB) ItemsForRunDate(ctx context.Context, topicID int64, runDate string) ([]Item, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, topic_id, url, title, source, published_at, summary, reason, run_date, created_at
		FROM items WHERE topic_id = ? AND run_date = ?
		ORDER BY COALESCE(published_at, created_at) DESC, id`, topicID, runDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		var pub, summary sql.NullString
		var created string
		if err := rows.Scan(&it.ID, &it.TopicID, &it.URL, &it.Title, &it.Source, &pub, &summary, &it.Reason, &it.RunDate, &created); err != nil {
			return nil, err
		}
		if pub.Valid {
			p := parseTime(pub.String)
			it.PublishedAt = &p
		}
		if summary.Valid && strings.TrimSpace(summary.String) != "" {
			s := summary.String
			it.Summary = &s
		}
		it.CreatedAt = parseTime(created)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (d *DB) RecordRun(ctx context.Context, r Run) error {
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO runs (topic_id, started_at, ok, error, n_results, n_kept) VALUES (?, ?, ?, ?, ?, ?)`,
		r.TopicID, fmtTime(r.StartedAt), r.OK, r.Error, r.NResults, r.NKept)
	return err
}

// DeleteItemsForRunDate removes a topic's items for one run date (used when its rules change).
func (d *DB) DeleteItemsForRunDate(ctx context.Context, topicID int64, runDate string) error {
	_, err := d.sql.ExecContext(ctx, `DELETE FROM items WHERE topic_id = ? AND run_date = ?`, topicID, runDate)
	return err
}
