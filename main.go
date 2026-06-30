// Command richmond-council-rss scrapes the Richmond upon Thames council news
// pages and serves them as an RSS 2.0 feed.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/dcelasun/richmond-council-rss/internal/scraper"
	"github.com/dcelasun/richmond-council-rss/internal/server"
	"github.com/dcelasun/richmond-council-rss/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := LoadConfig(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Error("configuration error", "err", err)
		os.Exit(2)
	}

	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := openStore(cfg, log)
	if err != nil {
		return err
	}
	defer st.Close()

	sc, err := scraper.New(scraper.Config{
		Store:     st,
		Logger:    log,
		NewsURL:   cfg.NewsURL,
		UserAgent: cfg.UserAgent,
		MaxItems:  cfg.MaxItems,
	})
	if err != nil {
		return err
	}

	handler := server.New(server.Options{
		Store:       st,
		Title:       "Richmond upon Thames Council News",
		Description: "Unofficial RSS feed for richmond.gov.uk/news",
		SiteLink:    cfg.NewsURL,
		FeedURL:     cfg.FeedURL,
		Language:    "en-GB",
		PageSize:    cfg.PageSize,
	})

	go func() {
		if err := sc.Run(ctx, cfg.ScrapeInterval); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("scraper stopped", "err", err)
		}
	}()

	log.Info("listening", "addr", cfg.ListenAddr, "store", storeKind(cfg))
	if err := server.ListenAndServe(ctx, cfg.ListenAddr, handler); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func openStore(cfg Config, log *slog.Logger) (store.Store, error) {
	if cfg.DBPath == "" {
		return store.NewMemory(), nil
	}
	log.Info("using sqlite store", "path", cfg.DBPath)
	return store.NewSQLite(cfg.DBPath)
}

func storeKind(cfg Config) string {
	if cfg.DBPath == "" {
		return "memory"
	}
	return "sqlite"
}
