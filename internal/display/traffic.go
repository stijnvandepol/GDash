package display

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/stijnvandepol/gdash/internal/traffic"
)

// TrafficSource is the data key of what Traffic widgets show: query key to
// jams, roadworks, speed checks and fixed cameras (traffic.Result).
const TrafficSource = "traffic.default"

// CamerasSource is the data key of the fixed camera list's summary; the
// cameras themselves reach screens through TrafficSource.
const CamerasSource = "traffic.cameras"

// KNMISource is the data key of the latest KNMI station observation, which
// Weather widgets set to show measured values use.
const KNMISource = "knmi.observation"

// Kinds of widget setting for the Traffic widget.
const (
	// SettingRoads is a list of distinct A and N roads, such as ["A2"].
	SettingRoads = "roads"
	// SettingChoices is a list of distinct Options, at least Min.
	SettingChoices = "choices"
)

// The values of a Weather widget's current setting: the weather model's
// values, or those a KNMI station measured.
const (
	CurrentModel = "model"
	CurrentKNMI  = "knmi"
)

// trafficWidget is a Traffic widget: roads to watch, or without roads a
// radius around the weather location, and what to show.
var trafficWidget = WidgetType{Source: TrafficSource, MinW: 6, MinH: 4, Settings: map[string]Setting{
	"roads":    {Kind: SettingRoads, Max: traffic.MaxRoads},
	"radius":   {Kind: SettingInteger, Min: traffic.MinRadiusKm, Max: traffic.MaxRadiusKm},
	"show":     {Kind: SettingChoices, Min: 1, Options: traffic.Kinds},
	"maxItems": {Kind: SettingInteger, Min: 1, Max: traffic.MaxItems},
}}

// weatherWidget is the Weather widget; it can show what the nearest KNMI
// station measured instead of the model's current values.
var weatherWidget = WidgetType{Source: WeatherSource, MinW: 4, MinH: 4, Settings: map[string]Setting{
	"current": {Kind: SettingChoice, Options: []string{CurrentModel, CurrentKNMI}},
}}

// roadList reads a roads setting: 1 to MaxRoads distinct valid roads, in
// capitals. ok is false for anything else.
func roadList(value any) ([]string, bool) {
	return idList(value, traffic.MaxRoads, traffic.ValidRoad)
}

func roadsProblem(key string, value any) string {
	if roads, ok := roadList(value); !ok || len(roads) == 0 {
		return fmt.Sprintf("%s must be 1 to %d roads such as A2 or N279, each once", key, traffic.MaxRoads)
	}
	return ""
}

func choicesProblem(key string, value any, rule Setting) string {
	chosen, ok := idList(value, len(rule.Options), func(s string) bool { return slices.Contains(rule.Options, s) })
	if !ok || len(chosen) < rule.Min {
		return fmt.Sprintf("%s must be at least %d of: %s, each once", key, rule.Min, strings.Join(rule.Options, ", "))
	}
	return ""
}

// trafficProblem returns why a Traffic widget's settings do not fit
// together, or "": it watches roads or a radius, never both.
func trafficProblem(settings map[string]any) string {
	_, hasRoads := settings["roads"]
	_, hasRadius := settings["radius"]
	if hasRoads && hasRadius {
		return "a traffic widget watches roads or a radius around the weather location, not both"
	}
	return ""
}

// trafficQuery reads what a Traffic widget watches, with the default radius
// for a widget without roads. ok is false for anything but a valid Traffic
// widget.
func trafficQuery(w Widget) (traffic.Query, bool) {
	if w.Type != "traffic" {
		return traffic.Query{}, false
	}
	if value, set := w.Settings["roads"]; set {
		roads, ok := roadList(value)
		if !ok || len(roads) == 0 {
			return traffic.Query{}, false
		}
		// Sorted as the key on the screen is.
		roads = slices.Clone(roads)
		slices.Sort(roads)
		return traffic.Query{Roads: roads}, true
	}
	q := traffic.Query{RadiusKm: traffic.DefaultRadiusKm}
	if value, set := w.Settings["radius"]; set {
		km, ok := number(value)
		if !ok || km != math.Trunc(km) || km < traffic.MinRadiusKm || km > traffic.MaxRadiusKm {
			return traffic.Query{}, false
		}
		q.RadiusKm = int(km)
	}
	return q, true
}

// TrafficQueries returns what the Traffic widgets on the dashboards watch,
// sorted and without repeats.
func TrafficQueries(dashboards []Dashboard) []traffic.Query {
	var all []traffic.Query
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if q, ok := trafficQuery(w); ok {
				all = append(all, q)
			}
		}
	}
	slices.SortFunc(all, func(a, b traffic.Query) int { return cmp.Compare(a.Key(), b.Key()) })
	return slices.CompactFunc(all, func(a, b traffic.Query) bool { return a.Key() == b.Key() })
}

// TrafficKeys returns the keys of TrafficQueries.
func TrafficKeys(dashboards []Dashboard) []string {
	queries := TrafficQueries(dashboards)
	keys := make([]string, len(queries))
	for i, q := range queries {
		keys[i] = q.Key()
	}
	return keys
}

// TrafficShows reports whether a Traffic widget on the dashboards shows
// kind, counting the default kinds for a widget without a show setting.
func TrafficShows(dashboards []Dashboard, kind string) bool {
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type != "traffic" {
				continue
			}
			shown := traffic.DefaultShow
			if value, set := w.Settings["show"]; set {
				shown, _ = idList(value, len(traffic.Kinds), func(string) bool { return true })
			}
			if slices.Contains(shown, kind) {
				return true
			}
		}
	}
	return false
}

// ShowsMeasuredWeather reports whether a Weather widget on the dashboards
// shows what a KNMI station measured.
func ShowsMeasuredWeather(dashboards []Dashboard) bool {
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type == "weather" && w.Settings["current"] == CurrentKNMI {
				return true
			}
		}
	}
	return false
}
