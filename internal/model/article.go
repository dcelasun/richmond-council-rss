// Package model defines the core data types shared across the scraper, store,
// feed and server packages.
package model

import (
	"net/url"
	"strings"
	"time"
)

// Article is a single news post extracted from the council website.
type Article struct {
	ID        string // stable per-post key, e.g. "news_june_2026/some_slug"
	URL       string // absolute; also the RSS <guid isPermaLink="true">
	Title     string
	Category  string
	Summary   string
	BodyHTML  string // sanitized full body with absolute image/link URLs
	Published time.Time
	FetchedAt time.Time // drives Last-Modified
}

// IDFromURL derives Article.ID by stripping the leading "/news/" from an article
// URL or path, e.g. "/news/news_june_2026/foo" -> "news_june_2026/foo".
func IDFromURL(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		raw = u.Path
	}
	raw = strings.Trim(raw, "/")
	return strings.TrimPrefix(raw, "news/")
}
