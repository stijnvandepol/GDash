package display

import (
	"encoding/json"
	"slices"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
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
	narrowShared(used, dashboards)
	return used
}

// onlyKeys narrows an entry that holds an object to the keys in ids: the
// Home Assistant entities, the transit stops, or the calendars of waste
// collection widgets. The server fetches those of every dashboard in one
// entry each, but a
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

// narrowShared cuts the entries that hold the data of every dashboard down
// to what the dashboards of one screen show.
func narrowShared(used map[string]datahub.Entry, dashboards []Dashboard) {
	if entry, ok := used[HomeAssistantSource]; ok {
		used[HomeAssistantSource] = onlyKeys(entry, EntityIDs(dashboards))
	}
	if entry, ok := used[WasteSource]; ok {
		used[WasteSource] = onlyCalendars(entry, WasteCalendarIDs(dashboards))
	}
	if entry, ok := used[DeparturesSource]; ok {
		used[DeparturesSource] = onlyKeys(entry, StopCodes(dashboards))
	}
	if entry, ok := used[FuelSource]; ok {
		used[FuelSource] = onlyKeys(entry, FuelKeys(dashboards))
	}
}

// entityIDs reads an entities setting: a list of distinct entity IDs, at
// most limit long. ok is false for anything else.
func entityIDs(value any, limit int) ([]string, bool) {
	list, ok := value.([]any)
	if !ok || len(list) > limit {
		return nil, false
	}
	ids := make([]string, 0, len(list))
	for _, v := range list {
		id, isText := v.(string)
		if !isText || !homeassistant.ValidEntityID(id) || slices.Contains(ids, id) {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

// EntityIDs returns every Home Assistant entity the dashboards show, sorted
// and without repeats, so the server fetches each one once.
func EntityIDs(dashboards []Dashboard) []string {
	var all []string
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if ids, ok := entityIDs(w.Settings["entities"], MaxEntitiesPerWidget); ok && w.Type == "homeassistant" {
				all = append(all, ids...)
			}
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// entityIDs reads an entities setting: a list of distinct entity IDs, at
// most limit long. ok is false for anything else.
func entityIDs(value any, limit int) ([]string, bool) {
	list, ok := value.([]any)
	if !ok || len(list) > limit {
		return nil, false
	}
	ids := make([]string, 0, len(list))
	for _, v := range list {
		id, isText := v.(string)
		if !isText || !homeassistant.ValidEntityID(id) || slices.Contains(ids, id) {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

// EntityIDs returns every Home Assistant entity the dashboards show, sorted
// and without repeats, so the server fetches each one once.
func EntityIDs(dashboards []Dashboard) []string {
	var all []string
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if ids, ok := entityIDs(w.Settings["entities"], MaxEntitiesPerWidget); ok && w.Type == "homeassistant" {
				all = append(all, ids...)
			}
		}
	}
	slices.Sort(all)
	return slices.Compact(all)
}
