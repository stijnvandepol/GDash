package news

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// The categories of the catalogue. web/src/admin/newsCatalogue.ts names
// them in the picker.
var Categories = []string{"general", "international", "regional", "sport", "economy", "tech", "science", "culture", "offbeat", "kids"}

// CatalogueEntry is a news feed that GDash offers to choose from: one that
// its publisher offers publicly as RSS or Atom, checked to answer with
// recent headlines when it was added.
type CatalogueEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Language is an ISO 639-1 code and Country an ISO 3166-1 alpha-2 code.
	Language string `json:"language"`
	Country  string `json:"country"`
	Category string `json:"category"`
	URL      string `json:"url"`
	// Note says what is known of the publisher's terms, or where it lists
	// its feeds; empty when nothing beyond the public feed is known.
	Note string `json:"note,omitempty"`
}

// catalogueJSON is reviewed like code: every entry was fetched once before
// it was added, see docs/guide.md#news.
//
//go:embed catalogue.json
var catalogueJSON []byte

var catalogue = mustParseCatalogue(catalogueJSON)

// mustParseCatalogue panics on a broken catalogue: it is part of the
// binary, so a mistake in it is a programmer error that the tests catch.
func mustParseCatalogue(data []byte) []CatalogueEntry {
	var entries []CatalogueEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		panic(fmt.Sprintf("news catalogue: %v", err))
	}
	return entries
}

// Catalogue returns the built-in news feeds, Dutch ones first.
func Catalogue() []CatalogueEntry {
	return append([]CatalogueEntry(nil), catalogue...)
}

// CatalogueEntryByURL returns the entry with the feed address url.
func CatalogueEntryByURL(url string) (CatalogueEntry, bool) {
	for _, e := range catalogue {
		if e.URL == url {
			return e, true
		}
	}
	return CatalogueEntry{}, false
}
