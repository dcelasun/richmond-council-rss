package store

import (
	"context"
	"testing"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
)

func art(id string, day int, fetched time.Time) model.Article {
	return model.Article{
		ID:        id,
		URL:       "https://x/" + id,
		Title:     id,
		Published: time.Date(2026, 6, day, 0, 0, 0, 0, time.UTC),
		FetchedAt: fetched,
	}
}

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()

	t0 := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	// Insert out of date order.
	for _, a := range []model.Article{
		art("b", 10, t0),
		art("a", 20, t0.Add(time.Hour)),
		art("c", 15, t0.Add(-time.Hour)),
	} {
		if err := m.Upsert(ctx, a); err != nil {
			t.Fatal(err)
		}
	}

	if n, _ := m.Count(ctx); n != 3 {
		t.Fatalf("count = %d, want 3", n)
	}

	has, _ := m.Has(ctx, "a")
	if !has {
		t.Error("Has(a) = false")
	}
	if has, _ := m.Has(ctx, "z"); has {
		t.Error("Has(z) = true")
	}

	// Page should return newest-first by Published: a(20), c(15), b(10).
	page, _ := m.Page(ctx, 10, 0)
	gotOrder := []string{page[0].ID, page[1].ID, page[2].ID}
	want := []string{"a", "c", "b"}
	for i := range want {
		if gotOrder[i] != want[i] {
			t.Errorf("order = %v, want %v", gotOrder, want)
			break
		}
	}

	// Pagination window.
	if p, _ := m.Page(ctx, 1, 1); len(p) != 1 || p[0].ID != "c" {
		t.Errorf("Page(1,1) = %+v, want [c]", p)
	}

	// LastModified is the max FetchedAt (article a).
	lm, _ := m.LastModified(ctx)
	if want := t0.Add(time.Hour); !lm.Equal(want) {
		t.Errorf("LastModified = %v, want %v", lm, want)
	}

	// Upsert of an existing ID updates rather than duplicates.
	if err := m.Upsert(ctx, art("a", 20, t0.Add(2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if n, _ := m.Count(ctx); n != 3 {
		t.Errorf("count after re-upsert = %d, want 3", n)
	}
}
