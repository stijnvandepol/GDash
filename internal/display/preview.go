package display

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/stijnvandepol/nookboard/internal/datahub"
)

// PreviewSnapshot is what a screen would be sent for dashboard d right now,
// for the preview in the editor. It never goes through the Broadcaster: no
// display, schedule or active hours apply, and the dashboard may be a draft
// that is not stored.
//
// Version is a hash of the content instead of a counter, so a page that
// polls can tell that nothing changed and leave what it shows alone.
func PreviewSnapshot(now time.Time, d Dashboard, data map[string]datahub.Entry, language string, textScale int) (Snapshot, error) {
	s := Snapshot{
		Type: "snapshot", Active: true, Dashboard: d,
		Data: dataFor([]Dashboard{d}, data), Language: language, TextScale: textScale,
	}
	// The hash leaves the server time out: it differs on every call.
	content, err := json.Marshal(s)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode preview of dashboard %s: %w", d.ID, err)
	}
	hash := fnv.New32a()
	_, _ = hash.Write(content)
	s.Version = int64(hash.Sum32())
	s.ServerTime = now
	return s, nil
}
