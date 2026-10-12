package display

import (
	"encoding/json"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// dataFor returns the entries that the widgets of the dashboards show.
func dataFor(dashboards []Dashboard, data map[string]datahub.Entry) map[string]datahub.Entry {
	used := map[string]datahub.Entry{}
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if entry, ok := data[w.Source]; ok && w.Source != "" {
				used[w.Source] = entry
			}
		}
	}
	if entry, ok := used[HomeAssistantSource]; ok {
		used[HomeAssistantSource] = onlyKeys(entry, EntityIDs(dashboards))
	}
	if entry, ok := used[WasteSource]; ok {
		used[WasteSource] = onlyCalendars(entry, WasteCalendarIDs(dashboards))
	}
	return used
}

// onlyKeys narrows an entry that holds an object to the keys in ids: the
// Home Assistant entities, or the calendars of waste collection widgets.
// The server fetches those of every dashboard in one entry, but a
// display may only receive what its own widgets show: a hall screen with a
// temperature must not get the door locks of the living room dashboard.
// An entry that cannot be read is replaced by one without a value, so the
// widget waits instead of showing more than it should.
func onlyKeys(entry datahub.Entry, ids []string) datahub.Entry {
	if len(entry.Value) == 0 {
		return entry
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(entry.Value, &all); err != nil {
		entry.Value = nil
		return entry
	}
	kept := make(map[string]json.RawMessage, len(ids))
	for _, id := range ids {
		if state, ok := all[id]; ok {
			kept[id] = state
		}
	}
	value, err := json.Marshal(kept)
	if err != nil {
		entry.Value = nil
		return entry
	}
	entry.Value = value
	return entry
}
