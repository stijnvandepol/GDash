package display

import (
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

const (
	// WebPageWidget is the type of the Web page widget.
	WebPageWidget = "webpage"
	// WebPageSource is the data key of the web pages under Integrations, by
	// ID. No source fetches it: the server adds it from the settings, and
	// each screen only receives the pages its own widgets show.
	WebPageSource = "webpage.default"
	// SettingWebPage is the ID of a web page under Integrations.
	SettingWebPage = "webPage"
)

// webPageEntryValidity only has to outlast the minute between publishes,
// which build the entry again.
const webPageEntryValidity = time.Hour

var webPageWidget = WidgetType{Source: WebPageSource, MinW: 4, MinH: 3, Settings: map[string]Setting{
	"page": {Kind: SettingWebPage, Required: true},
	// The page's own zoom applies when the widget has none.
	"zoom": {Kind: SettingInteger, Min: webpage.MinZoom, Max: webpage.MaxZoom},
}}

// shownPage is what a screen receives of a web page: what its frame needs.
type shownPage struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	RefreshMinutes int    `json:"refreshMinutes"`
	Zoom           int    `json:"zoom"`
	AllowScripts   bool   `json:"allowScripts"`
}

// WebPageEntry is the data entry of every web page, by ID. dataFor cuts it
// down to the pages of one screen's widgets.
func WebPageEntry(pages []webpage.Page, now time.Time) (datahub.Entry, error) {
	shown := make(map[string]shownPage, len(pages))
	for _, p := range pages {
		shown[strconv.FormatInt(p.ID, 10)] = shownPage{
			Name: p.Name, URL: p.URL, RefreshMinutes: p.RefreshMinutes, Zoom: p.Zoom, AllowScripts: p.AllowScripts,
		}
	}
	value, err := json.Marshal(shown)
	if err != nil {
		return datahub.Entry{}, err
	}
	return datahub.Entry{Value: value, FetchedAt: now, StaleAfter: now.Add(webPageEntryValidity)}, nil
}

// WebPageIDs returns the web pages that Web page widgets on the dashboards
// show, sorted and without repeats.
func WebPageIDs(dashboards []Dashboard) []int64 {
	var ids []int64
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if id, ok := recordID(w.Settings["page"]); ok && w.Type == WebPageWidget {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// FrameOrigins returns the origins of the web pages that the dashboards
// show, for the frame-src of a page that shows them.
func FrameOrigins(dashboards []Dashboard, pages []webpage.Page, ownOrigin string) []string {
	ids := WebPageIDs(dashboards)
	shown := slices.DeleteFunc(slices.Clone(pages), func(p webpage.Page) bool {
		_, found := slices.BinarySearch(ids, p.ID)
		return !found
	})
	return PageOrigins(shown, ownOrigin)
}

// PageOrigins returns the origins of pages, sorted and without repeats. A
// page on GDash's own origin is left out: a frame that runs scripts with
// its own origin could reach out of its sandbox. Integrations refuses such a
// page, but GDash can be reached on more than one address.
func PageOrigins(pages []webpage.Page, ownOrigin string) []string {
	var origins []string
	for _, p := range pages {
		origin, ok := webpage.Origin(p.URL)
		if ok && !webpage.IsOwnOrigin(origin, ownOrigin) {
			origins = append(origins, origin)
		}
	}
	slices.Sort(origins)
	return slices.Compact(origins)
}
