package display

import (
	"fmt"
	"slices"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

const (
	// NewsSource is the data key of the headlines of every news feed under
	// Integrations, by feed ID.
	NewsSource = "news.default"
	// SettingNewsFeeds is a list of IDs of news feeds under Integrations;
	// absent or empty means every feed.
	SettingNewsFeeds = "newsFeeds"
	// MaxNewsFeedsPerWidget is the number of feeds GDash keeps
	// (internal/settings.MaxNewsFeeds): a widget cannot show more.
	MaxNewsFeedsPerWidget = 10
)

// newsWidget shows the headlines of the feeds in feeds, or of every feed
// when the setting is absent, as dashboards from before the setting do.
var newsWidget = WidgetType{Source: NewsSource, MinW: 6, MinH: 4, Settings: map[string]Setting{
	"maxItems": {Kind: SettingInteger, Min: 1, Max: 20},
	"feeds":    {Kind: SettingNewsFeeds, Max: MaxNewsFeedsPerWidget},
}}

// newsFeedIDs reads a feeds setting: absent (nil) or a list of distinct
// feed IDs, at most MaxNewsFeedsPerWidget. ok is false for anything else.
func newsFeedIDs(value any) ([]int64, bool) {
	if value == nil {
		return nil, true
	}
	list, ok := value.([]any)
	if !ok || len(list) > MaxNewsFeedsPerWidget {
		return nil, false
	}
	ids := make([]int64, 0, len(list))
	for _, v := range list {
		id, ok := recordID(v)
		if !ok || slices.Contains(ids, id) {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func newsFeedsProblem(key string, value any) string {
	if _, ok := newsFeedIDs(value); !ok {
		return fmt.Sprintf("%s must be at most %d IDs of news feeds under Integrations, each once, or empty for every feed", key, MaxNewsFeedsPerWidget)
	}
	return ""
}

// NewsFeedIDs returns the news feeds that News widgets on the dashboards
// show, sorted and without repeats, and whether one of the widgets shows
// every feed.
func NewsFeedIDs(dashboards []Dashboard) (ids []int64, everyFeed bool) {
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type != "news" {
				continue
			}
			chosen, ok := newsFeedIDs(w.Settings["feeds"])
			if !ok {
				continue
			}
			if len(chosen) == 0 {
				everyFeed = true
			}
			ids = append(ids, chosen...)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), everyFeed
}

// onlyNews narrows the headlines to the feeds that the News widgets of one
// screen show: the study's screen with Tweakers does not receive the
// kitchen's NOS.
func onlyNews(entry datahub.Entry, dashboards []Dashboard) datahub.Entry {
	ids, everyFeed := NewsFeedIDs(dashboards)
	if everyFeed {
		return entry
	}
	return onlyIDs(entry, ids)
}
