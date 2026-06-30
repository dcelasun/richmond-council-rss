package scraper

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// articlePathRE matches an individual article path. Month-archive links like
// "/news/news_may_2026" (no trailing slug) deliberately do not match.
var articlePathRE = regexp.MustCompile(`^/news/news_[a-z]+_\d{4}/.+`)

var fullDateRE = regexp.MustCompile(`^\d{1,2} [A-Z][a-z]+ \d{4}$`)

const (
	fullDateLayout    = "2 January 2006" // article page: "30 June 2026"
	listingDateLayout = "2 Jan 06"       // listing page: "30 Jun 26"
)

type listingItem struct {
	URL       string
	Title     string
	Summary   string
	Category  string
	Published time.Time
}

// parseListing extracts the news items from a listing/archive page and the URL
// of the "View older news" archive page (empty if none). base resolves relative
// links to absolute.
func parseListing(r io.Reader, base *url.URL) ([]listingItem, string, error) {
	root, err := html.Parse(r)
	if err != nil {
		return nil, "", fmt.Errorf("parse listing: %w", err)
	}

	var items []listingItem
	var olderURL string
	var cur *listingItem // item currently being populated as we walk

	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.DataAtom == atom.H2:
				if a := firstArticleAnchor(n); a != nil {
					items = append(items, listingItem{
						URL:   resolveURL(base, getAttr(a, "href")),
						Title: strings.TrimSpace(textContent(a)),
					})
					cur = &items[len(items)-1]
				}
			case hasClass(n, "news-info"):
				if cur != nil {
					cur.Category, cur.Published = parseNewsInfo(textContent(n))
				}
			case hasClass(n, "col-lg-8"): // the summary column
				if cur != nil && cur.Summary == "" {
					cur.Summary = strings.TrimSpace(textContent(n))
				}
			case hasClass(n, "more_news"):
				if a := findFirst(n, func(c *html.Node) bool {
					return c.DataAtom == atom.A && getAttr(c, "href") != ""
				}); a != nil {
					olderURL = resolveURL(base, getAttr(a, "href"))
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	return items, olderURL, nil
}

func firstArticleAnchor(n *html.Node) *html.Node {
	return findFirst(n, func(c *html.Node) bool {
		if c.DataAtom != atom.A {
			return false
		}
		href := getAttr(c, "href")
		if href == "" {
			return false
		}
		if u, err := url.Parse(href); err == nil {
			return articlePathRE.MatchString(u.Path)
		}
		return false
	})
}

func parseNewsInfo(s string) (category string, published time.Time) {
	parts := strings.SplitN(s, "|", 2)
	category = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		if t, err := time.Parse(listingDateLayout, strings.TrimSpace(parts[1])); err == nil {
			published = t.UTC()
		}
	}
	return category, published
}

// extractArticle parses an article page into its headline, sanitized body HTML
// (with absolute image/link URLs) and publication date. pageURL is the absolute
// URL the page was fetched from, used to resolve relative links.
func extractArticle(r io.Reader, pageURL *url.URL) (title, bodyHTML string, published time.Time, err error) {
	root, err := html.Parse(r)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("parse article: %w", err)
	}

	if h1 := findFirst(root, func(n *html.Node) bool { return n.DataAtom == atom.H1 }); h1 != nil {
		title = strings.TrimSpace(textContent(h1))
	}

	published = findArticleDate(root)

	body := findFirst(root, func(n *html.Node) bool {
		return n.DataAtom == atom.Div && hasClass(n, "col-lg-7") && underNewsMain(n)
	})
	if body == nil {
		return title, "", published, fmt.Errorf("article body container not found")
	}

	absolutizeURLs(body, pageURL)
	bodyHTML, err = renderChildren(body)
	if err != nil {
		return title, "", published, err
	}
	bodyHTML = strings.TrimSpace(bodyHTML)
	return title, bodyHTML, published, nil
}

// findArticleDate returns the first text node matching the article-page date
// format. The publication date precedes the body, so the first match wins
// (later "Updated:" / in-body dates have prefixes and don't match).
func findArticleDate(root *html.Node) time.Time {
	var found time.Time
	var walk func(n *html.Node) bool
	walk = func(n *html.Node) bool {
		if n.Type == html.TextNode {
			s := strings.TrimSpace(n.Data)
			if fullDateRE.MatchString(s) {
				if t, err := time.Parse(fullDateLayout, s); err == nil {
					found = t.UTC()
					return true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if walk(c) {
				return true
			}
		}
		return false
	}
	walk(root)
	return found
}

func underNewsMain(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.DataAtom == atom.Div && hasClass(p, "newsmain") {
			return true
		}
	}
	return false
}

func absolutizeURLs(n *html.Node, base *url.URL) {
	if n.Type == html.ElementNode {
		switch n.DataAtom {
		case atom.Img:
			setAttr(n, "src", resolveURL(base, getAttr(n, "src")))
		case atom.A:
			if getAttr(n, "href") != "" {
				setAttr(n, "href", resolveURL(base, getAttr(n, "href")))
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		absolutizeURLs(c, base)
	}
}

func renderChildren(n *html.Node) (string, error) {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		// Skip empty <p> the parser leaves behind when auto-closing the source's
		// nested <p><p>.
		if c.Type == html.ElementNode && c.DataAtom == atom.P && isEmptyNode(c) {
			continue
		}
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func isEmptyNode(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode:
			return false
		case html.TextNode:
			if strings.TrimSpace(c.Data) != "" {
				return false
			}
		}
	}
	return true
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(getAttr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// findFirst returns the first descendant of n (document order, excluding n
// itself) satisfying pred, or nil.
func findFirst(n *html.Node, pred func(*html.Node) bool) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if pred(c) {
			return c
		}
		if got := findFirst(c, pred); got != nil {
			return got
		}
	}
	return nil
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(u).String()
}
