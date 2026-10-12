package display

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/homey"
)

const (
	// HomeySource is the data key of the capabilities that Homey widgets
	// show: "<device id>/<capability id>" to its state.
	HomeySource = "homey.default"
	// HomeyUsersSource is the data key of whether the users that Who's
	// home widgets look for are home. It never reaches a screen as it is:
	// each screen gets HomeyPresenceSource instead.
	HomeyUsersSource = "homey.users"
	// HomeyPresenceSource is the data key of the labels of the people who
	// are home, by widget (CustomKey). It is built for each screen from
	// HomeyUsersSource and holds nobody who is away.
	HomeyPresenceSource = "homey.presence"

	// HomeyPresenceWidget is the type of the Who's home widget.
	HomeyPresenceWidget = "homeypresence"

	// MaxHomeyCapabilities keeps a Homey widget readable from across a
	// room, like a Home Assistant widget.
	MaxHomeyCapabilities = 8
	// MaxHomeyUsers is the most people a Who's home widget looks for.
	MaxHomeyUsers = 8
	// maxBadgeLabel keeps a badge a circle or a short pill.
	maxBadgeLabel = 12
)

// Kinds of the Homey widgets' settings.
const (
	// SettingHomeyCapabilities is a list of capability keys; Min and Max
	// bound how many.
	SettingHomeyCapabilities = "homeyCapabilities"
	// SettingHomeyUsers is a list of {id, label}: a Homey user and the
	// short label, such as initials, that shows while that user is home.
	SettingHomeyUsers = "homeyUsers"
)

var homeyWidget = WidgetType{Source: HomeySource, MinW: 4, MinH: 3, Settings: map[string]Setting{
	"capabilities": {Kind: SettingHomeyCapabilities, Min: 1, Max: MaxHomeyCapabilities, Required: true},
}}

var homeyPresenceWidget = WidgetType{Source: HomeyPresenceSource, MinW: 4, MinH: 3, Settings: map[string]Setting{
	"users": {Kind: SettingHomeyUsers, Min: 1, Max: MaxHomeyUsers, Required: true},
}}

// presenceUser is one person a Who's home widget looks for.
type presenceUser struct {
	id, label string
}

// presenceUsers reads a users setting: 1 to MaxHomeyUsers distinct users,
// each exactly an id and a label. ok is false for anything else.
func presenceUsers(value any) ([]presenceUser, bool) {
	list, ok := value.([]any)
	if !ok || len(list) == 0 || len(list) > MaxHomeyUsers {
		return nil, false
	}
	users := make([]presenceUser, 0, len(list))
	for _, v := range list {
		fields, isObject := v.(map[string]any)
		id, hasID := fields["id"].(string)
		label, hasLabel := fields["label"].(string)
		n := utf8.RuneCountInString(label)
		if !isObject || len(fields) != 2 || !hasID || !homey.ValidUserID(id) || !hasLabel ||
			n < 1 || n > maxBadgeLabel || strings.TrimSpace(label) != label || hasControl(label) || strings.Contains(label, "\n") ||
			slices.ContainsFunc(users, func(u presenceUser) bool { return u.id == id }) {
			return nil, false
		}
		users = append(users, presenceUser{id: id, label: label})
	}
	return users, true
}

func homeySettingProblem(key string, value any, rule Setting) string {
	if rule.Kind == SettingHomeyUsers {
		if _, ok := presenceUsers(value); !ok {
			return fmt.Sprintf("%s must be 1 to %d Homey users, each with a label of 1 to %d characters", key, MaxHomeyUsers, maxBadgeLabel)
		}
		return ""
	}
	if keys, ok := idList(value, rule.Max, homey.ValidKey); !ok || len(keys) < rule.Min {
		return fmt.Sprintf("choose %d to %d Homey device values", rule.Min, rule.Max)
	}
	return ""
}

// HomeyCapabilityKeys returns every capability the dashboards' Homey
// widgets show, sorted and without repeats.
func HomeyCapabilityKeys(dashboards []Dashboard) []string {
	var all []string
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if keys, ok := idList(w.Settings["capabilities"], MaxHomeyCapabilities, homey.ValidKey); ok && w.Type == "homey" {
				all = append(all, keys...)
			}
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// presenceWidget is a valid Who's home widget with its key in
// HomeyPresenceSource.
type presenceWidget struct {
	key   string
	users []presenceUser
}

func presenceWidgets(dashboards []Dashboard) []presenceWidget {
	var out []presenceWidget
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if users, ok := presenceUsers(w.Settings["users"]); ok && w.Type == HomeyPresenceWidget {
				out = append(out, presenceWidget{key: CustomKey(d.ID, w.ID), users: users})
			}
		}
	}
	return out
}

// HomeyUserIDs returns every user the dashboards' Who's home widgets look
// for, sorted and without repeats, so only their presence is kept.
func HomeyUserIDs(dashboards []Dashboard) []string {
	var all []string
	for _, w := range presenceWidgets(dashboards) {
		for _, u := range w.users {
			all = append(all, u.id)
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// presenceEntry builds what the Who's home widgets of the dashboards show:
// for each widget, the labels of its users who are home, in its order. It
// reports false when they have none or Homey is not set up. A screen never
// learns who is away: that is neither in this entry nor, see
// screenDashboard, in its layout.
func presenceEntry(dashboards []Dashboard, data map[string]datahub.Entry) (datahub.Entry, bool) {
	widgets := presenceWidgets(dashboards)
	source, ok := data[HomeyUsersSource]
	if len(widgets) == 0 || !ok {
		return datahub.Entry{}, false
	}
	entry := datahub.Entry{FetchedAt: source.FetchedAt, StaleAfter: source.StaleAfter, Failing: source.Failing}
	if len(source.Value) == 0 {
		return entry, true
	}
	var present map[string]bool
	if err := json.Unmarshal(source.Value, &present); err != nil {
		entry.Failing = true
		return entry, true
	}
	shown := make(map[string][]string, len(widgets))
	for _, w := range widgets {
		labels := []string{}
		for _, u := range w.users {
			if present[u.id] {
				labels = append(labels, u.label)
			}
		}
		shown[w.key] = labels
	}
	value, err := json.Marshal(shown)
	if err != nil {
		entry.Failing = true
		return entry, true
	}
	entry.Value = value
	return entry, true
}

// screenDashboard returns d as a screen receives it: Who's home widgets
// without the users they look for, whose labels the screen gets from
// HomeyPresenceSource for those who are home only.
func screenDashboard(d Dashboard) Dashboard {
	if !slices.ContainsFunc(d.Widgets, func(w Widget) bool { return w.Type == HomeyPresenceWidget }) {
		return d
	}
	widgets := make([]Widget, len(d.Widgets))
	for i, w := range d.Widgets {
		if w.Type == HomeyPresenceWidget {
			w.Settings = nil
		}
		widgets[i] = w
	}
	d.Widgets = widgets
	return d
}

// screenDashboards applies screenDashboard to each of list.
func screenDashboards(list []Dashboard) []Dashboard {
	if list == nil {
		return nil
	}
	out := make([]Dashboard, len(list))
	for i, d := range list {
		out[i] = screenDashboard(d)
	}
	return out
}
