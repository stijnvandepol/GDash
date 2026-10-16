package p2000

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/stijnvandepol/gdash/internal/news"
)

type rssDocument struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// Parse reads a feed of alarmeringen.nl and returns at most MaxPerFeed of
// its alerts, in feed order, all with discipline. An item without a date or
// a text is left out: it cannot be placed in time or shown.
func Parse(r io.Reader, discipline string) ([]Alert, error) {
	items, err := parseItems(r)
	if err != nil {
		return nil, err
	}
	alerts := []Alert{}
	for _, it := range items {
		if len(alerts) == MaxPerFeed {
			break
		}
		if a, ok := alertOf(it, discipline); ok {
			alerts = append(alerts, a)
		}
	}
	return alerts, nil
}

func parseItems(r io.Reader) ([]rssItem, error) {
	var doc rssDocument
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("not an RSS feed: %w", err)
	}
	return doc.Channel.Items, nil
}

func alertOf(it rssItem, discipline string) (Alert, bool) {
	at := news.ParseRSSDate(it.PubDate).UTC()
	// The readable sentence; the title is the raw pager text, a fallback.
	text := news.PlainText(it.Description)
	if text == "" {
		text = news.PlainText(it.Title)
	}
	if at.IsZero() || at.Year() > 9999 || text == "" {
		return Alert{}, false
	}
	_, place := regionAndPlace(it.Link)
	id := news.PlainText(it.GUID)
	if id == "" {
		id = at.Format("20060102150405") + " " + text
	}
	return Alert{ID: id, At: at, Discipline: discipline, Place: place, Text: text}, true
}

// regionAndPlace reads the region and place slugs from an alert's link:
// /{province}/{region}/{place}/{id}/p2000-….html. Fryslân has no province
// level (/friesland/{place}/…), and some items link to the place's page
// without an id. Either slug is "" when the link does not say.
func regionAndPlace(link string) (region, place string) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return "", ""
	}
	var parts []string
	for p := range strings.SplitSeq(u.Path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	// The place is the last slug before the id, or the last part of a
	// place page's link.
	end := len(parts)
	for i, p := range parts {
		if isNumber(p) {
			end = i
			break
		}
	}
	if end == 0 || !ValidSlug(parts[end-1]) {
		return "", ""
	}
	place = parts[end-1]
	if end >= 2 && ValidRegion(parts[end-2]) {
		region = parts[end-2]
	}
	return region, place
}

func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
