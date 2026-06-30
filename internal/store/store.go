// Package store provides persistence for scraped articles. Two implementations
// are available: an in-memory store and a SQLite-backed store. Both satisfy the
// Store interface and return articles newest-first.
package store

import (
	"context"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
)

type Store interface {
	Has(ctx context.Context, id string) (bool, error)
	Upsert(ctx context.Context, a model.Article) error
	// Page returns up to limit articles ordered by Published descending,
	// skipping the first offset.
	Page(ctx context.Context, limit, offset int) ([]model.Article, error)
	Count(ctx context.Context) (int, error)
	// LastModified returns the most recent FetchedAt, or the zero time when the
	// store is empty. It drives the feed's Last-Modified header.
	LastModified(ctx context.Context) (time.Time, error)
	Close() error
}
