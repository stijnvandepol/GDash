package custom

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"
)

// SourceKey is the data hub key of a source's answer. No widget type has
// it as its source, so the raw answer never reaches a screen; screens get
// the Shown values of their own widgets instead.
func SourceKey(id int64) string { return "custom.source." + strconv.FormatInt(id, 10) }

// Documents decodes the answers of the sources and keeps the result until
// a new answer arrives. Every snapshot for every screen reads them, and an
// answer may be a megabyte.
type Documents struct {
	mu   sync.Mutex
	docs map[string]document
}

type document struct {
	fetchedAt time.Time
	size      int
	value     any
}

// maxDocuments is above the number of sources, so the cache only drops a
// document that no source has any more.
const maxDocuments = 2 * MaxSources

// Decode returns raw decoded. key and fetchedAt identify one answer of one
// source; a new answer decodes again.
func (d *Documents) Decode(key string, raw json.RawMessage, fetchedAt time.Time) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if doc, ok := d.docs[key]; ok && doc.fetchedAt.Equal(fetchedAt) && doc.size == len(raw) {
		return doc.value, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	if d.docs == nil || len(d.docs) >= maxDocuments {
		d.docs = map[string]document{}
	}
	d.docs[key] = document{fetchedAt: fetchedAt, size: len(raw), value: value}
	return value, nil
}
