// Package scraper fetches the council news pages and extracts articles.
package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
	"github.com/dcelasun/richmond-council-rss/internal/store"
)

type Scraper struct {
	client    *http.Client
	store     store.Store
	log       *slog.Logger
	newsURL   *url.URL
	userAgent string
	maxItems  int
	delay     time.Duration
}

type Config struct {
	Store     store.Store
	Logger    *slog.Logger
	NewsURL   string
	UserAgent string
	MaxItems  int
	Client    *http.Client
	// Delay paces successive page fetches to be polite during backfill.
	Delay time.Duration
}

func New(cfg Config) (*Scraper, error) {
	news, err := url.Parse(cfg.NewsURL)
	if err != nil {
		return nil, fmt.Errorf("parse news URL %q: %w", cfg.NewsURL, err)
	}

	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	delay := cfg.Delay
	if delay == 0 {
		delay = 500 * time.Millisecond
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Scraper{
		client:    client,
		store:     cfg.Store,
		log:       log,
		newsURL:   news,
		userAgent: cfg.UserAgent,
		maxItems:  cfg.MaxItems,
		delay:     delay,
	}, nil
}

// Run backfills when the store is empty, then scrapes the front page on each
// interval tick until ctx is cancelled.
func (s *Scraper) Run(ctx context.Context, interval time.Duration) error {
	n, err := s.store.Count(ctx)
	if err != nil {
		return fmt.Errorf("count store: %w", err)
	}
	if n == 0 {
		if err := s.Backfill(ctx); err != nil {
			s.log.Error("initial backfill failed", "err", err)
		}
	} else {
		s.log.Info("store already populated, skipping backfill", "count", n)
		if err := s.RunOnce(ctx); err != nil {
			s.log.Error("scrape failed", "err", err)
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil {
				s.log.Error("scrape failed", "err", err)
			}
		}
	}
}

func (s *Scraper) RunOnce(ctx context.Context) error {
	items, _, err := s.scrapeListing(ctx, s.newsURL)
	if err != nil {
		return err
	}
	added, err := s.storeNew(ctx, items)
	if err != nil {
		return err
	}
	s.log.Info("scrape complete", "found", len(items), "added", added)
	return nil
}

// Backfill walks the monthly-archive chain from the front page until maxItems
// new articles have been stored or the chain ends.
func (s *Scraper) Backfill(ctx context.Context) error {
	s.log.Info("starting backfill", "max_items", s.maxItems)
	pageURL := s.newsURL
	total := 0
	for pageURL != nil {
		items, older, err := s.scrapeListing(ctx, pageURL)
		if err != nil {
			return err
		}
		for _, it := range items {
			if total >= s.maxItems {
				s.log.Info("backfill complete", "stored", total)
				return nil
			}
			added, err := s.storeNew(ctx, []listingItem{it})
			if err != nil {
				return err
			}
			total += added
		}
		if older == "" {
			break
		}
		next, err := url.Parse(older)
		if err != nil {
			break
		}
		pageURL = next
		if err := s.sleep(ctx); err != nil {
			return err
		}
	}
	s.log.Info("backfill complete (archive exhausted)", "stored", total)
	return nil
}

func (s *Scraper) storeNew(ctx context.Context, items []listingItem) (int, error) {
	added := 0
	for _, it := range items {
		id := model.IDFromURL(it.URL)
		has, err := s.store.Has(ctx, id)
		if err != nil {
			return added, err
		}
		if has {
			continue
		}
		a, err := s.fetchArticle(ctx, it)
		if err != nil {
			s.log.Warn("failed to fetch article", "url", it.URL, "err", err)
			continue
		}
		if err := s.store.Upsert(ctx, a); err != nil {
			return added, err
		}
		added++
		if err := s.sleep(ctx); err != nil {
			return added, err
		}
	}
	return added, nil
}

func (s *Scraper) scrapeListing(ctx context.Context, pageURL *url.URL) ([]listingItem, string, error) {
	body, err := s.fetch(ctx, pageURL.String())
	if err != nil {
		return nil, "", err
	}
	defer body.Close()
	return parseListing(body, s.newsURL)
}

func (s *Scraper) fetchArticle(ctx context.Context, it listingItem) (model.Article, error) {
	pageURL, err := url.Parse(it.URL)
	if err != nil {
		return model.Article{}, err
	}
	body, err := s.fetch(ctx, it.URL)
	if err != nil {
		return model.Article{}, err
	}
	defer body.Close()
	title, bodyHTML, published, err := extractArticle(body, pageURL)
	if err != nil {
		return model.Article{}, err
	}
	if title == "" {
		title = it.Title
	}
	if published.IsZero() {
		published = it.Published
	}
	return model.Article{
		ID:        model.IDFromURL(it.URL),
		URL:       it.URL,
		Title:     title,
		Category:  it.Category,
		Summary:   it.Summary,
		BodyHTML:  bodyHTML,
		Published: published,
		FetchedAt: time.Now().UTC(),
	}, nil
}

func (s *Scraper) fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if s.userAgent != "" {
		req.Header.Set("User-Agent", s.userAgent)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	return resp.Body, nil
}

func (s *Scraper) sleep(ctx context.Context) error {
	t := time.NewTimer(s.delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
