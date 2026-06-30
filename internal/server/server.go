// Package server exposes the stored articles as an RSS feed over HTTP, with
// pagination and conditional-GET (Last-Modified / ETag) support.
package server

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/feed"
	"github.com/dcelasun/richmond-council-rss/internal/store"
)

type Options struct {
	Store       store.Store
	Title       string
	Description string
	SiteLink    string
	FeedURL     string // canonical feed URL for atom:self and paging links
	Language    string
	PageSize    int
}

type Handler struct {
	opts Options
}

func New(opts Options) http.Handler {
	if opts.PageSize <= 0 {
		opts.PageSize = 50
	}
	h := &Handler{opts: opts}
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", h.serveFeed)
	mux.HandleFunc("/rss", h.serveFeed)
	mux.HandleFunc("/healthz", h.serveHealth)
	mux.HandleFunc("/", h.serveRoot)
	return mux
}

func (h *Handler) serveRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	h.serveFeed(w, r)
}

func (h *Handler) serveHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) serveFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	page := pageParam(r)
	pageSize := h.opts.PageSize

	total, err := h.opts.Store.Count(ctx)
	if err != nil {
		h.serverError(w, "count", err)
		return
	}
	lastMod, err := h.opts.Store.LastModified(ctx)
	if err != nil {
		h.serverError(w, "last-modified", err)
		return
	}
	articles, err := h.opts.Store.Page(ctx, pageSize, (page-1)*pageSize)
	if err != nil {
		h.serverError(w, "page", err)
		return
	}

	out, err := feed.Render(feed.Metadata{
		Title:       h.opts.Title,
		SiteLink:    h.opts.SiteLink,
		FeedURL:     h.opts.FeedURL,
		Description: h.opts.Description,
		Language:    h.opts.Language,
		LastBuilt:   lastMod,
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
	}, articles)
	if err != nil {
		h.serverError(w, "render", err)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=300")
	w.Header().Set("ETag", etag(lastMod, page, pageSize, total))
	// http.ServeContent applies If-None-Match (against the ETag set above) and
	// If-Modified-Since (against lastMod), emitting 304 and Last-Modified.
	http.ServeContent(w, r, "feed.xml", lastMod, bytes.NewReader(out))
}

func (h *Handler) serverError(w http.ResponseWriter, op string, err error) {
	http.Error(w, fmt.Sprintf("%s: %v", op, err), http.StatusInternalServerError)
}

func pageParam(r *http.Request) int {
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
		return p
	}
	return 1
}

// etag derives a strong ETag from the inputs that determine the response body.
func etag(lastMod time.Time, page, pageSize, total int) string {
	hsh := fnv.New64a()
	fmt.Fprintf(hsh, "%d|%d|%d|%d", lastMod.UnixNano(), page, pageSize, total)
	return fmt.Sprintf(`"%x"`, hsh.Sum64())
}

// ListenAndServe runs an HTTP server with sane timeouts until ctx is cancelled,
// then gracefully shuts it down.
func ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
