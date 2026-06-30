package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"

	_ "modernc.org/sqlite"
)

// SQLite is a SQLite-backed Store using the pure-Go modernc.org/sqlite driver.
type SQLite struct {
	db *sql.DB
}

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS articles (
	id         TEXT PRIMARY KEY,
	url        TEXT NOT NULL,
	title      TEXT NOT NULL,
	category   TEXT NOT NULL,
	summary    TEXT NOT NULL,
	body       TEXT NOT NULL,
	published  INTEGER NOT NULL, -- unix seconds
	fetched_at INTEGER NOT NULL  -- unix seconds
);
CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published DESC);
`

func NewSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// modernc's driver serializes writes; a single connection avoids
	// SQLITE_BUSY under our low-concurrency workload.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (s *SQLite) Has(ctx context.Context, id string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM articles WHERE id = ?`, id).Scan(&one)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func (s *SQLite) Upsert(ctx context.Context, a model.Article) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO articles (id, url, title, category, summary, body, published, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			url=excluded.url, title=excluded.title, category=excluded.category,
			summary=excluded.summary, body=excluded.body,
			published=excluded.published, fetched_at=excluded.fetched_at`,
		a.ID, a.URL, a.Title, a.Category, a.Summary, a.BodyHTML,
		a.Published.Unix(), a.FetchedAt.Unix())
	return err
}

func (s *SQLite) Page(ctx context.Context, limit, offset int) ([]model.Article, error) {
	if limit <= 0 {
		limit = -1 // SQLite treats negative LIMIT as "no limit"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, url, title, category, summary, body, published, fetched_at
		FROM articles ORDER BY published DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Article
	for rows.Next() {
		var a model.Article
		var pub, fetched int64
		if err := rows.Scan(&a.ID, &a.URL, &a.Title, &a.Category, &a.Summary,
			&a.BodyHTML, &pub, &fetched); err != nil {
			return nil, err
		}
		a.Published = time.Unix(pub, 0).UTC()
		a.FetchedAt = time.Unix(fetched, 0).UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *SQLite) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles`).Scan(&n)
	return n, err
}

func (s *SQLite) LastModified(ctx context.Context) (time.Time, error) {
	var ts sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(fetched_at) FROM articles`).Scan(&ts); err != nil {
		return time.Time{}, err
	}
	if !ts.Valid {
		return time.Time{}, nil
	}
	return time.Unix(ts.Int64, 0).UTC(), nil
}

func (s *SQLite) Close() error { return s.db.Close() }
