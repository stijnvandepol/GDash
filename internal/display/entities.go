package display

import (
	"encoding/json"

	"github.com/stijnvandepol/nookboard/internal/datahub"
)

// onlyEntities narrows the Home Assistant entry to the entities in ids.
// The server fetches the entities of every dashboard in one entry, but a
// display may only receive what its own widgets show: a hall screen with a
// temperature must not get the door locks of the living room dashboard.
// An entry that cannot be read is replaced by one without a value, so the
// widget waits instead of showing more than it should.
func onlyEntities(entry datahub.Entry, ids []string) datahub.Entry {
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
