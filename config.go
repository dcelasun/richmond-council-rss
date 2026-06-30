package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	ListenAddr     string
	ScrapeInterval time.Duration
	DBPath         string
	MaxItems       int
	PageSize       int
	NewsURL        string
	FeedURL        string
	UserAgent      string
}

// LoadConfig parses flags, then overlays environment variables, which take
// priority over flags.
func LoadConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("richmond-council-rss", flag.ContinueOnError)

	listen := fs.String("listen", ":8080", "HTTP listen address (env RSS_LISTEN_ADDR)")
	interval := fs.Duration("interval", 30*time.Minute, "scrape interval (env RSS_SCRAPE_INTERVAL)")
	db := fs.String("db", "", "SQLite file path; empty = in-memory (env RSS_DB_PATH)")
	maxItems := fs.Int("max-items", 50, "number of posts to backfill on first run (env RSS_MAX_ITEMS)")
	pageSize := fs.Int("page-size", 50, "items per feed page (env RSS_PAGE_SIZE)")
	siteURL := fs.String("site-url", "https://richmond.gov.uk/news", "news listing URL (env RSS_SITE_URL)")
	feedURL := fs.String("feed-url", "http://localhost:8080/feed.xml", "public feed URL for atom:self/paging (env RSS_FEED_URL)")
	userAgent := fs.String("user-agent", "richmond-council-rss/1.0 (+https://github.com/dcelasun/richmond-council-rss)", "scraper User-Agent (env RSS_USER_AGENT)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:     envOr("RSS_LISTEN_ADDR", *listen),
		ScrapeInterval: *interval,
		DBPath:         envOr("RSS_DB_PATH", *db),
		MaxItems:       *maxItems,
		PageSize:       *pageSize,
		NewsURL:        envOr("RSS_SITE_URL", *siteURL),
		FeedURL:        envOr("RSS_FEED_URL", *feedURL),
		UserAgent:      envOr("RSS_USER_AGENT", *userAgent),
	}

	// Non-string settings are parsed explicitly when supplied via env.
	if v := os.Getenv("RSS_SCRAPE_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("RSS_SCRAPE_INTERVAL: %w", err)
		}
		cfg.ScrapeInterval = d
	}
	if v, err := envInt("RSS_MAX_ITEMS", *maxItems); err != nil {
		return Config{}, err
	} else {
		cfg.MaxItems = v
	}
	if v, err := envInt("RSS_PAGE_SIZE", *pageSize); err != nil {
		return Config{}, err
	} else {
		cfg.PageSize = v
	}

	if cfg.ScrapeInterval <= 0 {
		return Config{}, fmt.Errorf("interval must be positive")
	}
	if cfg.MaxItems <= 0 {
		return Config{}, fmt.Errorf("max-items must be positive")
	}
	if cfg.PageSize <= 0 {
		return Config{}, fmt.Errorf("page-size must be positive")
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q", key, v)
	}
	return n, nil
}
