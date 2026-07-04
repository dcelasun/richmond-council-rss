// Package feed renders stored articles as an RSS 2.0 document with full article
// bodies in <content:encoded> and RFC 5005 paging links.
package feed

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/dcelasun/richmond-council-rss/internal/model"
)

const (
	nsContent = "http://purl.org/rss/1.0/modules/content/"
	nsAtom    = "http://www.w3.org/2005/Atom"
	generator = "richmond-council-rss"
)

type Metadata struct {
	Title       string
	SiteLink    string
	FeedURL     string // canonical feed URL without query; paging adds ?page=N
	Description string
	Language    string
	LastBuilt   time.Time
	Page        int // 1-based
	PageSize    int
	Total       int
}

// cdata wraps a string so encoding/xml emits it inside a CDATA section.
type cdata struct {
	Value string `xml:",cdata"`
}

type rss struct {
	XMLName   xml.Name `xml:"rss"`
	Version   string   `xml:"version,attr"`
	ContentNS string   `xml:"xmlns:content,attr"`
	AtomNS    string   `xml:"xmlns:atom,attr"`
	Channel   channel  `xml:"channel"`
}

type channel struct {
	Title         string     `xml:"title"`
	Link          string     `xml:"link"`
	Description   string     `xml:"description"`
	Language      string     `xml:"language,omitempty"`
	LastBuildDate string     `xml:"lastBuildDate,omitempty"`
	Generator     string     `xml:"generator"`
	AtomLinks     []atomLink `xml:"atom:link"`
	Items         []item     `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
}

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        guid   `xml:"guid"`
	PubDate     string `xml:"pubDate,omitempty"`
	Category    string `xml:"category,omitempty"`
	Description cdata  `xml:"description"`
	Content     cdata  `xml:"content:encoded"`
}

type guid struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// Render builds the RSS 2.0 document (including XML declaration) for one page of
// articles.
func Render(meta Metadata, articles []model.Article) ([]byte, error) {
	ch := channel{
		Title:       meta.Title,
		Link:        meta.SiteLink,
		Description: meta.Description,
		Language:    meta.Language,
		Generator:   generator,
		AtomLinks:   pagingLinks(meta),
	}
	if !meta.LastBuilt.IsZero() {
		ch.LastBuildDate = meta.LastBuilt.UTC().Format(time.RFC1123Z)
	}

	for _, a := range articles {
		it := item{
			Title:       a.Title,
			Link:        a.URL,
			GUID:        guid{IsPermaLink: true, Value: a.URL},
			Category:    a.Category,
			Description: cdata{Value: a.Summary},
			Content:     cdata{Value: a.BodyHTML},
		}
		if !a.Published.IsZero() {
			it.PubDate = a.Published.UTC().Format(time.RFC1123Z)
		}
		ch.Items = append(ch.Items, it)
	}

	doc := rss{
		Version:   "2.0",
		ContentNS: nsContent,
		AtomNS:    nsAtom,
		Channel:   ch,
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

// pagingLinks builds the atom:link self/first/previous/next/last set per
// RFC 5005, omitting next/previous at the boundaries.
func pagingLinks(meta Metadata) []atomLink {
	links := []atomLink{{
		Href: pageURL(meta.FeedURL, meta.Page),
		Rel:  "self",
		Type: "application/rss+xml",
	}}
	if meta.PageSize <= 0 {
		return links
	}
	last := max((meta.Total+meta.PageSize-1)/meta.PageSize, 1)
	links = append(links,
		atomLink{Href: pageURL(meta.FeedURL, 1), Rel: "first"},
		atomLink{Href: pageURL(meta.FeedURL, last), Rel: "last"},
	)
	if meta.Page > 1 {
		links = append(links, atomLink{Href: pageURL(meta.FeedURL, meta.Page-1), Rel: "previous"})
	}
	if meta.Page < last {
		links = append(links, atomLink{Href: pageURL(meta.FeedURL, meta.Page+1), Rel: "next"})
	}
	return links
}

// pageURL appends a ?page=N query to base (page 1 is left bare).
func pageURL(base string, page int) string {
	if page <= 1 {
		return base
	}
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Sprintf("%s?page=%d", base, page)
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String()
}
