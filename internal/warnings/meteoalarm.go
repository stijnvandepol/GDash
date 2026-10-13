package warnings

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is MeteoAlarm's feed of the KNMI's warnings.
const DefaultBaseURL = "https://feeds.meteoalarm.org/api/v1/warnings/feeds-netherlands"

// maxFeedBytes bounds what is read. The feed is about 1 MB on a calm day,
// with every province's "no warnings" in two languages; a stormy day adds
// alerts with outlines of the coast.
const maxFeedBytes = 16 << 20

// Client reads the MeteoAlarm feed.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// UserAgent names GDash to MeteoAlarm.
	UserAgent string
	Now       func() time.Time
}

// ForRegions returns the warnings of each query's region, by the query's
// Key. The feed is read once for all of them. A region without warnings has
// an empty list.
func (c Client) ForRegions(ctx context.Context, queries []Query) (map[string]Area, error) {
	for _, q := range queries {
		if !ValidRegion(q.Region) {
			return nil, fmt.Errorf("warnings query %q has no known region", q.Key())
		}
	}
	feed, err := c.feed(ctx)
	if err != nil {
		return nil, err
	}
	all, err := parse(feed, c.Now())
	if err != nil {
		return nil, err
	}
	out := make(map[string]Area, len(queries))
	for _, q := range queries {
		out[q.Key()] = Area{Region: q.Region, Warnings: all[q.Region]}
	}
	return out, nil
}

func (c Client) feed(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build meteoalarm request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request meteoalarm: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("meteoalarm answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read meteoalarm feed: %w", err)
	}
	if len(body) > maxFeedBytes {
		return nil, fmt.Errorf("meteoalarm feed is larger than %d bytes", maxFeedBytes)
	}
	return body, nil
}

// The part of a CAP alert in MeteoAlarm's JSON that GDash reads.
type feed struct {
	Warnings *[]struct {
		Alert *capAlert `json:"alert"`
	} `json:"warnings"`
}

type capAlert struct {
	Identifier string    `json:"identifier"`
	Sent       string    `json:"sent"`
	Status     string    `json:"status"`
	MsgType    string    `json:"msgType"`
	References string    `json:"references"`
	Info       []capInfo `json:"info"`
}

type capInfo struct {
	Language  string     `json:"language"`
	Event     string     `json:"event"`
	Headline  string     `json:"headline"`
	Effective string     `json:"effective"`
	Onset     string     `json:"onset"`
	Expires   string     `json:"expires"`
	Parameter []capValue `json:"parameter"`
	Area      []struct {
		AreaDesc string     `json:"areaDesc"`
		Geocode  []capValue `json:"geocode"`
	} `json:"area"`
}

// capValue is a named value: a parameter or a geocode.
type capValue struct {
	ValueName string `json:"valueName"`
	Value     string `json:"value"`
}

// parse reads the feed into the warnings per region that have not expired
// at now, soonest first. It keeps real alerts and their updates only (not
// tests, exercises or cancellations) at code yellow and up. An alert that
// cannot be read is left out, so one broken alert does not hide the rest;
// a feed without a warnings list is an error, so the screen keeps the last
// good one instead of saying all is calm.
func parse(body []byte, now time.Time) (map[string][]Warning, error) {
	var f feed
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("decode meteoalarm feed: %w", err)
	}
	if f.Warnings == nil {
		return nil, fmt.Errorf("meteoalarm feed has no warnings list")
	}
	latest := map[string]map[string]sentWarning{}
	for _, entry := range *f.Warnings {
		a := entry.Alert
		if a == nil || a.Status != "Actual" || (a.MsgType != "Alert" && a.MsgType != "Update") {
			continue
		}
		w, ok := readAlert(*a)
		if !ok || !now.Before(w.Expires) {
			continue
		}
		// An alert without a readable sent time counts as the oldest.
		sent, err := time.Parse(time.RFC3339, a.Sent)
		if err != nil {
			sent = time.Time{}
		}
		for _, name := range alertRegions(*a) {
			if latest[name] == nil {
				latest[name] = map[string]sentWarning{}
			}
			// An update that is still in the feed next to the alert it
			// replaced: the newest one counts.
			if prev, seen := latest[name][w.ID]; !seen || sent.After(prev.sent) {
				latest[name][w.ID] = sentWarning{w, sent}
			}
		}
	}
	out := make(map[string][]Warning, len(latest))
	for name, byID := range latest {
		list := make([]Warning, 0, len(byID))
		for _, sw := range byID {
			list = append(list, sw.warning)
		}
		slices.SortFunc(list, func(a, b Warning) int {
			return cmp.Or(a.Onset.Compare(b.Onset), cmp.Compare(levelRank(b.Level), levelRank(a.Level)), cmp.Compare(a.ID, b.ID))
		})
		out[name] = list
	}
	return out, nil
}

type sentWarning struct {
	warning Warning
	sent    time.Time
}

// readAlert turns an alert into a warning. Its languages are infos that
// share the level and times; the first info that has them decides them.
func readAlert(a capAlert) (Warning, bool) {
	w := Warning{ID: stableID(a), Event: map[string]string{}, Headline: map[string]string{}}
	found := false
	for _, info := range a.Info {
		language, _, _ := strings.Cut(strings.ToLower(info.Language), "-")
		if language != "" {
			setText(w.Event, language, info.Event)
			setText(w.Headline, language, info.Headline)
		}
		if found {
			continue
		}
		level, kind, onset, expires, ok := readInfo(info)
		if ok {
			w.Level, w.Kind, w.Onset, w.Expires, found = level, kind, onset, expires, true
		}
	}
	return w, found && w.ID != ""
}

// maxTextRunes keeps one long headline from filling a screen.
const maxTextRunes = 160

func setText(texts map[string]string, language, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if runes := []rune(text); len(runes) > maxTextRunes {
		text = string(runes[:maxTextRunes-1]) + "…"
	}
	texts[language] = text
}

func readInfo(info capInfo) (level, kind string, onset, expires time.Time, ok bool) {
	for _, p := range info.Parameter {
		switch p.ValueName {
		case "awareness_level":
			level = awarenessLevel(p.Value)
		case "awareness_type":
			kind = awarenessType(p.Value)
		}
	}
	start := info.Onset
	if start == "" {
		start = info.Effective
	}
	onset, onsetErr := time.Parse(time.RFC3339, start)
	expires, expiresErr := time.Parse(time.RFC3339, info.Expires)
	ok = level != "" && onsetErr == nil && expiresErr == nil && expires.After(onset)
	return level, kind, onset, expires, ok
}

// awarenessLevel reads "3; orange; Severe". Green and anything unknown
// give "".
func awarenessLevel(value string) string {
	number, _, _ := strings.Cut(value, ";")
	switch strings.TrimSpace(number) {
	case "2":
		return LevelYellow
	case "3":
		return LevelOrange
	case "4":
		return LevelRed
	}
	return ""
}

// awarenessTypes are MeteoAlarm's awareness types by number; 11 is unused.
var awarenessTypes = map[int]string{
	1: "wind", 2: "snow-ice", 3: "thunderstorm", 4: "fog", 5: "high-temperature", 6: "low-temperature",
	7: "coastal-event", 8: "forest-fire", 9: "avalanches", 10: "rain", 12: "flooding", 13: "rain-flood",
}

// awarenessType reads "1; Wind" by its number, so a change of wording does
// not matter.
func awarenessType(value string) string {
	number, _, _ := strings.Cut(value, ";")
	n, err := strconv.Atoi(strings.TrimSpace(number))
	if err != nil {
		return ""
	}
	return awarenessTypes[n]
}

func levelRank(level string) int {
	return slices.Index([]string{LevelYellow, LevelOrange, LevelRed}, level)
}

// stableID is the identifier of the first alert in a chain of updates: an
// update names the alerts it replaces in references, as
// "sender,identifier,sent" separated by spaces. Without references the
// alert is the first itself.
func stableID(a capAlert) string {
	first, firstSent := a.Identifier, time.Time{}
	for _, ref := range strings.Fields(a.References) {
		parts := strings.Split(ref, ",")
		if len(parts) != 3 || parts[1] == "" {
			continue
		}
		sent, err := time.Parse(time.RFC3339, parts[2])
		if err != nil {
			continue
		}
		if firstSent.IsZero() || sent.Before(firstSent) {
			first, firstSent = parts[1], sent
		}
	}
	return first
}

// alertRegions are the regions of GDash that an alert's areas name, by
// MeteoAlarm's EMMA_ID code or else by the area's name.
func alertRegions(a capAlert) []string {
	var names []string
	for _, info := range a.Info {
		for _, area := range info.Area {
			for _, r := range regions {
				if r.name == area.AreaDesc || hasEMMA(area.Geocode, r.emma) {
					names = append(names, r.name)
				}
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func hasEMMA(codes []capValue, emma string) bool {
	return slices.Contains(codes, capValue{ValueName: "EMMA_ID", Value: emma})
}
