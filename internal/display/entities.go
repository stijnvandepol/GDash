package display

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
)

// dataFor returns the entries that the widgets of the dashboards show at
// now.
func dataFor(dashboards []Dashboard, data map[string]datahub.Entry, now time.Time) map[string]datahub.Entry {
	used := map[string]datahub.Entry{}
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if entry, ok := data[w.Source]; ok && w.Source != "" {
				used[w.Source] = entry
			}
		}
	}
	narrowShared(used, dashboards)
	if entry, ok := customEntry(dashboards, data, now); ok {
		used[CustomSource] = entry
	}
	if entry, ok := presenceEntry(dashboards, data); ok {
		used[HomeyPresenceSource] = entry
	}
	return used
}

// onlyKeys narrows an entry that holds an object to the keys in ids: the
// Home Assistant entities, the Homey capabilities, the transit stops, the
// calendars of waste collection widgets, or the parts of the holidays
// report. The server fetches those of every dashboard in one
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

// keyed is a widget's query that names its part of a shared entry.
type keyed interface{ Key() string }

// keysOf returns the keys of queries, in their order.
func keysOf[Q keyed](queries []Q) []string {
	keys := make([]string, len(queries))
	for i, q := range queries {
		keys[i] = q.Key()
	}
	return keys
}

// uniqueByKey sorts queries by key and drops repeats, so the server asks
// for each once.
func uniqueByKey[Q keyed](queries []Q) []Q {
	slices.SortFunc(queries, func(a, b Q) int { return cmp.Compare(a.Key(), b.Key()) })
	return slices.CompactFunc(queries, func(a, b Q) bool { return a.Key() == b.Key() })
}

// narrowShared cuts the entries that hold the data of every dashboard down
// to what the dashboards of one screen show.
func narrowShared(used map[string]datahub.Entry, dashboards []Dashboard) {
	if entry, ok := used[HomeAssistantSource]; ok {
		used[HomeAssistantSource] = onlyKeys(entry, EntityIDs(dashboards))
	}
	if entry, ok := used[HomeySource]; ok {
		used[HomeySource] = onlyKeys(entry, HomeyCapabilityKeys(dashboards))
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
	if entry, ok := used[HolidaysSource]; ok {
		used[HolidaysSource] = onlyKeys(entry, HolidayKeys(dashboards))
	}
}

// entityIDs reads an entities setting: a list of distinct entity IDs, at
// most limit long. ok is false for anything else.
func entityIDs(value any, limit int) ([]string, bool) {
	return idList(value, limit, homeassistant.ValidEntityID)
}

// idList reads a setting that lists distinct IDs that valid accepts, at
// most limit. ok is false for anything else.
func idList(value any, limit int, valid func(string) bool) ([]string, bool) {
	list, ok := value.([]any)
	if !ok || len(list) > limit {
		return nil, false
	}
	ids := make([]string, 0, len(list))
	for _, v := range list {
		id, isText := v.(string)
		if !isText || !valid(id) || slices.Contains(ids, id) {
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
