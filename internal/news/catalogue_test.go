package news

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

var (
	validCatalogueID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	isoLanguage      = regexp.MustCompile(`^[a-z]{2}$`)
	isoCountry       = regexp.MustCompile(`^[A-Z]{2}$`)
)

// maxSourceName is the longest name internal/settings accepts for a feed.
const maxSourceName = 60

func TestCatalogueEntriesAreComplete(t *testing.T) {
	entries := Catalogue()
	if len(entries) < 10 {
		t.Fatalf("catalogue has %d entries; the embedded file did not load", len(entries))
	}
	ids, urls := map[string]bool{}, map[string]bool{}
	for _, e := range entries {
		if !validCatalogueID.MatchString(e.ID) {
			t.Errorf("%q: id must be lowercase letters, digits and dashes", e.ID)
		}
		if ids[e.ID] {
			t.Errorf("%q: id is used twice", e.ID)
		}
		ids[e.ID] = true
		if urls[e.URL] {
			t.Errorf("%s: url %s is used twice", e.ID, e.URL)
		}
		urls[e.URL] = true
		u, err := url.Parse(e.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
			t.Errorf("%s: url %q is not an http(s) address", e.ID, e.URL)
		}
		if name := strings.TrimSpace(e.Name); name == "" || name != e.Name || utf8.RuneCountInString(name) > maxSourceName {
			t.Errorf("%s: name %q must be 1 to %d characters without outer spaces", e.ID, e.Name, maxSourceName)
		}
		if !isoLanguage.MatchString(e.Language) || !isoCountry.MatchString(e.Country) {
			t.Errorf("%s: language %q or country %q is not an ISO code", e.ID, e.Language, e.Country)
		}
		if !slices.Contains(Categories, e.Category) {
			t.Errorf("%s: category %q is not one of %v", e.ID, e.Category, Categories)
		}
	}
}

func TestCatalogueListsDutchSourcesFirst(t *testing.T) {
	entries := Catalogue()
	firstOther := slices.IndexFunc(entries, func(e CatalogueEntry) bool { return e.Country != "NL" })
	if firstOther < 0 {
		t.Fatal("catalogue has only Dutch sources")
	}
	if slices.ContainsFunc(entries[firstOther:], func(e CatalogueEntry) bool { return e.Country == "NL" }) {
		t.Fatal("a Dutch source comes after a source from another country")
	}
}

func TestCatalogueEntryByURL(t *testing.T) {
	first := Catalogue()[0]
	if got, ok := CatalogueEntryByURL(first.URL); !ok || got.ID != first.ID {
		t.Fatalf("CatalogueEntryByURL(%q) = %+v, %v; want %s", first.URL, got, ok, first.ID)
	}
	if _, ok := CatalogueEntryByURL("https://feeds.example.invalid/own.xml"); ok {
		t.Fatal("an address of one's own is found in the catalogue")
	}
}

func TestCatalogueReturnsACopy(t *testing.T) {
	Catalogue()[0].Name = "changed"
	if Catalogue()[0].Name == "changed" {
		t.Fatal("a caller changed the catalogue")
	}
}
