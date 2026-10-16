// Package news reads RSS 2.0 and Atom feeds into plain headlines. Feed
// content is untrusted: titles are reduced to plain text and links to
// http(s) URLs, so a display never renders markup from a feed.
package news

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// maxTitleRunes cuts very long titles; a TV shows one or two lines.
const maxTitleRunes = 200

// Item is one headline.
type Item struct {
	Title     string    `json:"title"`
	Link      string    `json:"link,omitempty"`
	Published time.Time `json:"published,omitzero"`
}

// Feed is the name of a feed and its newest headlines.
type Feed struct {
	Title string `json:"title"`
	Items []Item `json:"items"`
}

type rssDocument struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title   string `xml:"title"`
			Link    string `xml:"link"`
			PubDate string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atomDocument struct {
	Title   string `xml:"title"`
	Entries []struct {
		Title string `xml:"title"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
	} `xml:"entry"`
}

// Parse reads an RSS 2.0 or Atom document and returns at most maxItems
// headlines, in feed order.
func Parse(r io.Reader, maxItems int) (Feed, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Feed{}, err
	}
	root, err := rootElement(data)
	if err != nil {
		return Feed{}, err
	}
	switch root {
	case "rss":
		return parseRSS(data, maxItems)
	case "feed":
		return parseAtom(data, maxItems)
	default:
		return Feed{}, fmt.Errorf("not an RSS or Atom feed: root element <%s>", root)
	}
}

func rootElement(data []byte) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("read feed: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local, nil
		}
	}
}

func parseRSS(data []byte, maxItems int) (Feed, error) {
	var doc rssDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Feed{}, fmt.Errorf("parse RSS: %w", err)
	}
	feed := Feed{Title: PlainText(doc.Channel.Title), Items: []Item{}}
	for _, it := range doc.Channel.Items {
		if len(feed.Items) == maxItems {
			break
		}
		if item, ok := newItem(it.Title, it.Link, ParseRSSDate(it.PubDate)); ok {
			feed.Items = append(feed.Items, item)
		}
	}
	return feed, nil
}

func parseAtom(data []byte, maxItems int) (Feed, error) {
	var doc atomDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Feed{}, fmt.Errorf("parse Atom: %w", err)
	}
	feed := Feed{Title: PlainText(doc.Title), Items: []Item{}}
	for _, e := range doc.Entries {
		if len(feed.Items) == maxItems {
			break
		}
		link := ""
		for _, l := range e.Links {
			if l.Rel == "" || l.Rel == "alternate" {
				link = l.Href
				break
			}
		}
		date := e.Published
		if date == "" {
			date = e.Updated
		}
		published, _ := time.Parse(time.RFC3339, strings.TrimSpace(date))
		if item, ok := newItem(e.Title, link, published); ok {
			feed.Items = append(feed.Items, item)
		}
	}
	return feed, nil
}

func newItem(title, link string, published time.Time) (Item, bool) {
	t := PlainText(title)
	if t == "" {
		return Item{}, false
	}
	return Item{Title: t, Link: safeLink(link), Published: inJSONRange(published.UTC())}, true
}

// inJSONRange returns the zero time for a date in year 0 or past 9999, which
// JSON cannot write: one such date would fail the snapshot of every display
// that shows the feed. An absurd date is as good as an unknown one.
func inJSONRange(t time.Time) time.Time {
	if t.Year() < 1 || t.Year() > 9999 {
		return time.Time{}
	}
	return t
}

// rssDateLayouts covers RFC 822 dates as feeds really write them: with and
// without a leading zero in the day (the NOS writes "Thu, 8 Oct 2026"),
// with a numeric zone or a zone name.
var rssDateLayouts = []string{
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04 -0700",
}

// ParseRSSDate returns the zero time when the date is missing or unknown;
// the headline is still worth showing without it.
func ParseRSSDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range rssDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// PlainText turns feed text into one line of plain text: tags removed,
// entities decoded, whitespace collapsed and long text cut.
func PlainText(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>' && inTag:
			inTag = false
			b.WriteRune(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	text := strings.Join(strings.Fields(html.UnescapeString(b.String())), " ")
	if utf8.RuneCountInString(text) > maxTitleRunes {
		text = string([]rune(text)[:maxTitleRunes-1]) + "…"
	}
	return text
}

// safeLink keeps only absolute http(s) links, so a javascript: or data: URL
// from a feed never reaches a page.
func safeLink(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}
