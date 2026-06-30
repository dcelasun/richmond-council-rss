package scraper

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestParseListing(t *testing.T) {
	f, err := os.Open("testdata/listing.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	site := mustParseURL(t, "https://richmond.gov.uk")
	items, older, err := parseListing(f, site)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("no items parsed")
	}

	first := items[0]
	if want := "https://richmond.gov.uk/news/news_june_2026/mayor_invites_residents_charity_reception"; first.URL != want {
		t.Errorf("first URL = %q, want %q", first.URL, want)
	}
	if !strings.Contains(first.Title, "Mayor of Richmond") {
		t.Errorf("first title = %q", first.Title)
	}
	if first.Category != "Council news" {
		t.Errorf("first category = %q, want %q", first.Category, "Council news")
	}
	if !strings.HasPrefix(first.Summary, "Residents are invited") {
		t.Errorf("first summary = %q", first.Summary)
	}
	wantDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	if !first.Published.Equal(wantDate) {
		t.Errorf("first published = %v, want %v", first.Published, wantDate)
	}

	if want := "https://richmond.gov.uk/news/news_may_2026"; older != want {
		t.Errorf("older link = %q, want %q", older, want)
	}

	// Every item should have a valid article URL (slug present).
	for i, it := range items {
		if !articlePathRE.MatchString(mustParseURL(t, it.URL).Path) {
			t.Errorf("item %d URL not an article path: %q", i, it.URL)
		}
	}
}

func TestExtractArticle(t *testing.T) {
	f, err := os.Open("testdata/article.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	pageURL := mustParseURL(t, "https://richmond.gov.uk/news/news_june_2026/mayor_invites_residents_charity_reception")
	title, body, published, err := extractArticle(f, pageURL)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(title, "Mayor of Richmond") {
		t.Errorf("title = %q", title)
	}
	wantDate := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	if !published.Equal(wantDate) {
		t.Errorf("published = %v, want %v", published, wantDate)
	}

	// Body must contain the article text and the event list.
	if !strings.Contains(body, "Asgill House") {
		t.Error("body missing expected text 'Asgill House'")
	}
	if !strings.Contains(body, "<ul>") || !strings.Contains(body, "Old Palace Lane") {
		t.Error("body missing event-details list")
	}

	// Images must be present with absolute URLs.
	if !strings.Contains(body, `<img src="https://richmond.gov.uk/media/5vhji5d2/asgill_house.jpg"`) {
		t.Error("body missing absolute image URL")
	}
	if strings.Contains(body, `src="/media/`) {
		t.Error("body still contains a relative image src")
	}

	// Relative article links must be absolutized; external links untouched.
	if strings.Contains(body, `href="/`) {
		t.Error("body still contains a root-relative href")
	}
	if !strings.Contains(body, `href="https://www.skylarks.charity/"`) {
		t.Error("external link altered or missing")
	}

	// Boilerplate (share buttons / sidebar) must be excluded.
	if strings.Contains(body, "Share this") || strings.Contains(body, "rrssb") {
		t.Error("body leaked share/sidebar boilerplate")
	}
}

func TestParseNewsInfo(t *testing.T) {
	cat, pub := parseNewsInfo("Council news\n| 30 Jun 26")
	if cat != "Council news" {
		t.Errorf("category = %q", cat)
	}
	if want := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC); !pub.Equal(want) {
		t.Errorf("published = %v, want %v", pub, want)
	}
}
