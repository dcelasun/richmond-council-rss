package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
	"github.com/dcelasun/richmond-council-rss/internal/store"
)

func newTestServer(t *testing.T) (http.Handler, time.Time) {
	t.Helper()
	st := store.NewMemory()
	fetched := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	_ = st.Upsert(context.Background(), model.Article{
		ID: "news_june_2026/a", URL: "https://x/a", Title: "A",
		BodyHTML: "<p>a</p>", Published: fetched, FetchedAt: fetched,
	})
	return New(Options{Store: st, Title: "T", FeedURL: "https://x/feed.xml", PageSize: 10}), fetched
}

func TestServeFeedOK(t *testing.T) {
	h, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/rss+xml; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("missing ETag")
	}
	if rec.Header().Get("Last-Modified") == "" {
		t.Error("missing Last-Modified")
	}
}

func TestConditionalGetIfModifiedSince(t *testing.T) {
	h, fetched := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	req.Header.Set("If-Modified-Since", fetched.UTC().Format(http.TimeFormat))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
}

func TestConditionalGetIfNoneMatch(t *testing.T) {
	h, _ := newTestServer(t)

	// First request to learn the ETag.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/feed.xml", nil))
	etag := rec.Header().Get("ETag")

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec2.Code)
	}
}

func TestHealth(t *testing.T) {
	h, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("health = %d %q", rec.Code, rec.Body.String())
	}
}
