package display

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// CustomSource is the data key of what Custom widgets show, by
// CustomKey. Unlike other keys, no source fetches it: each screen's entry
// is built from the answers of the custom sources for its own widgets.
const CustomSource = "custom.default"

// CustomWidget is the type of the Custom widget.
const CustomWidget = "custom"

// Kinds of the Custom widget's settings, for the editor; the custom
// package validates them.
const (
	// SettingCustomSource is the ID of a custom source under Settings.
	SettingCustomSource = "customSource"
	// SettingPath is a path into an answer, such as data.items[0].name.
	SettingPath = "path"
	// SettingPaths is a list of at most Max paths.
	SettingPaths = "paths"
	// SettingPairs is a list of at most Max {label, path}.
	SettingPairs = "pairs"
	// SettingRules is a list of at most Max {when, tone, message, hide, announce}.
	SettingRules = "rules"
)

var customWidget = WidgetType{Source: CustomSource, MinW: 4, MinH: 3, Settings: map[string]Setting{
	"source":   {Kind: SettingCustomSource, Required: true},
	"mode":     {Kind: SettingChoice, Options: custom.Modes, Required: true},
	"title":    {Kind: SettingText, Max: 60},
	"path":     {Kind: SettingPath},
	"label":    {Kind: SettingText, Max: 40},
	"unit":     {Kind: SettingText, Max: 12},
	"decimals": {Kind: SettingInteger, Min: 0, Max: custom.MaxDecimals},
	"fields":   {Kind: SettingPaths, Max: custom.MaxFields},
	"pairs":    {Kind: SettingPairs, Max: custom.MaxPairs},
	"rules":    {Kind: SettingRules, Max: custom.MaxRules},
}}

// customProblem returns the first problem with a Custom widget's settings,
// as "setting: message", or "".
func customProblem(settings map[string]any) string {
	_, err := custom.ParseWidget(settings)
	// ParseWidget checks in a fixed order, so a layout always gets the
	// same message.
	if fields := validation.Fields(err); len(fields) > 0 {
		return fields[0].Field + ": " + fields[0].Message
	}
	return ""
}

// CustomKey identifies a Custom widget in the CustomSource entry. Widget
// IDs are only unique within a dashboard, and a screen can show several.
func CustomKey(dashboardID, widgetID string) string { return dashboardID + "/" + widgetID }

// CustomSourceIDs returns the custom sources that the dashboards show,
// sorted and without repeats, so only those are fetched.
func CustomSourceIDs(dashboards []Dashboard) []int64 {
	var ids []int64
	for _, w := range customWidgets(dashboards) {
		ids = append(ids, w.widget.Source)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

type parsedCustom struct {
	key    string
	widget custom.Widget
}

// customWidgets returns the valid Custom widgets of the dashboards. A
// stored dashboard has only valid ones; a draft in the preview may not.
func customWidgets(dashboards []Dashboard) []parsedCustom {
	var out []parsedCustom
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type != CustomWidget {
				continue
			}
			if parsed, err := custom.ParseWidget(w.Settings); err == nil {
				out = append(out, parsedCustom{key: CustomKey(d.ID, w.ID), widget: parsed})
			}
		}
	}
	return out
}

// customDocuments keeps the decoded answers between snapshots.
var customDocuments custom.Documents

// CustomData returns a source's latest answer from the data hub's entries.
func CustomData(data map[string]datahub.Entry, sourceID int64, now time.Time) custom.Data {
	key := custom.SourceKey(sourceID)
	entry, ok := data[key]
	if !ok {
		return custom.Data{Missing: true}
	}
	d := custom.Data{Failing: entry.Failing, Stale: !entry.StaleAfter.IsZero() && now.After(entry.StaleAfter)}
	if len(entry.Value) == 0 {
		return d
	}
	doc, err := customDocuments.Decode(key, entry.Value, entry.FetchedAt)
	if err != nil {
		d.Failing = true
		return d
	}
	d.Doc, d.HasValue = doc, true
	return d
}

// customEntry builds what the Custom widgets of the dashboards show, or
// reports false when they have none.
func customEntry(dashboards []Dashboard, data map[string]datahub.Entry, now time.Time) (datahub.Entry, bool) {
	widgets := customWidgets(dashboards)
	if len(widgets) == 0 {
		return datahub.Entry{}, false
	}
	shown := make(map[string]custom.Shown, len(widgets))
	for _, w := range widgets {
		shown[w.key] = w.widget.Show(CustomData(data, w.widget.Source, now), now)
	}
	value, err := json.Marshal(shown)
	if err != nil {
		return datahub.Entry{Failing: true}, true
	}
	// Each widget carries its own state; the entry itself is never stale.
	return datahub.Entry{Value: value, FetchedAt: now, StaleAfter: now.Add(customEntryValidity)}, true
}

// customEntryValidity only has to outlast the minute between publishes,
// which build the entry again.
const customEntryValidity = time.Hour

// CustomTargets returns the Custom widgets of the dashboards that announce,
// with their source's answer, for the rules that post announcements.
// Widgets whose source has no answer yet are left out.
func CustomTargets(dashboards []Dashboard, data map[string]datahub.Entry, now time.Time) []custom.Target {
	var out []custom.Target
	for _, w := range customWidgets(dashboards) {
		if !slices.ContainsFunc(w.widget.Rules, func(r custom.Rule) bool { return r.Announce }) {
			continue
		}
		if d := CustomData(data, w.widget.Source, now); d.HasValue {
			out = append(out, custom.Target{Key: w.key, Widget: w.widget, Doc: d.Doc})
		}
	}
	return out
}
