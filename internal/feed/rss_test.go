package feed

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
)

func sampleArticles() []model.Article {
	return []model.Article{
		{
			ID:        "news_june_2026/a",
			URL:       "https://richmond.gov.uk/news/news_june_2026/a",
			Title:     "First & Best",
			Category:  "Council news",
			Summary:   "A short summary",
			BodyHTML:  `<p>Hello <img src="https://richmond.gov.uk/media/x.jpg"/></p>`,
			Published: time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC),
		},
		{
			ID:        "news_june_2026/b",
			URL:       "https://richmond.gov.uk/news/news_june_2026/b",
			Title:     "Second",
			Summary:   "Another",
			BodyHTML:  `<p>World</p>`,
			Published: time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC),
		},
	}
}

func baseMeta() Metadata {
	return Metadata{
		Title:       "Richmond News",
		SiteLink:    "https://richmond.gov.uk/news",
		FeedURL:     "https://example.com/feed.xml",
		Description: "Test feed",
		Language:    "en-GB",
		LastBuilt:   time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
		Page:        1,
		PageSize:    2,
		Total:       5,
	}
}

func TestRenderWellFormedAndContent(t *testing.T) {
	out, err := Render(baseMeta(), sampleArticles())
	if err != nil {
		t.Fatal(err)
	}

	// Must be well-formed XML.
	if err := xml.Unmarshal(out, new(struct {
		XMLName xml.Name `xml:"rss"`
	})); err != nil {
		t.Fatalf("not well-formed: %v", err)
	}

	s := string(out)
	for _, want := range []string{
		`xmlns:content="http://purl.org/rss/1.0/modules/content/"`,
		`xmlns:atom="http://www.w3.org/2005/Atom"`,
		"<content:encoded><![CDATA[<p>Hello",
		"<title>First &amp; Best</title>",
		`<guid isPermaLink="true">https://richmond.gov.uk/news/news_june_2026/a</guid>`,
		"<category>Council news</category>",
		"Mon, 29 Jun 2026 09:00:00 +0000", // pubDate of second item
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPagingLinks(t *testing.T) {
	// Page 2 of 3 (Total 5, PageSize 2 => 3 pages): expect first/last/prev/next.
	meta := baseMeta()
	meta.Page = 2
	out, _ := Render(meta, sampleArticles())
	s := string(out)

	checks := map[string]bool{
		`rel="self"`: true,
		`href="https://example.com/feed.xml" rel="first"`: true,
		`rel="last"`: true,
		`href="https://example.com/feed.xml" rel="previous"`: true, // page 1 is bare
		`page=3" rel="next"`: true,
	}
	for frag := range checks {
		if !strings.Contains(s, frag) {
			t.Errorf("page 2 output missing %q", frag)
		}
	}

	// Page 1 must not advertise a previous link.
	out1, _ := Render(baseMeta(), sampleArticles())
	if strings.Contains(string(out1), `rel="previous"`) {
		t.Error("page 1 should not have a previous link")
	}
}
