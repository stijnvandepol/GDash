package display

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

func TestOnlyScreensWithAnAlbumWidgetGetThatAlbum(t *testing.T) {
	photosFrom := func(id, source string) Dashboard {
		return Normalize(Dashboard{ID: id, Widgets: []Widget{{ID: "photos", Type: "photos", W: 6, H: 4, Settings: map[string]any{"source": source}}}})
	}
	immichPhotos, synologyPhotos := photosFrom("family", PhotosImmich), photosFrom("hall", PhotosSynology)
	uploaded := Normalize(Dashboard{ID: "kitchen", Widgets: []Widget{{ID: "photos", Type: "photos", W: 6, H: 4}}})
	if immichPhotos.Widgets[0].Source != ImmichSource || synologyPhotos.Widgets[0].Source != SynologySource || uploaded.Widgets[0].Source != PhotosSource {
		t.Fatalf("sources = %q, %q, %q", immichPhotos.Widgets[0].Source, synologyPhotos.Widgets[0].Source, uploaded.Widgets[0].Source)
	}
	if !UsesAlbum([]Dashboard{uploaded, immichPhotos}, ImmichSource) || UsesAlbum([]Dashboard{uploaded, immichPhotos}, SynologySource) {
		t.Fatal("UsesAlbum does not follow the Photos widgets")
	}
	data := map[string]datahub.Entry{
		ImmichSource:   {Value: json.RawMessage(`["4b1c5d6e-7f80-4912-a3b4-c5d6e7f80912"]`)},
		SynologySource: {Value: json.RawMessage(`["80716-80716_1633859830-xl"]`)},
		PhotosSource:   {Value: json.RawMessage(`[]`)},
	}
	if got := dataFor([]Dashboard{uploaded}, data, time.Now()); len(got) != 1 || got[PhotosSource].Value == nil {
		t.Errorf("a screen without an album gets %v", got)
	}
	if got := dataFor([]Dashboard{immichPhotos}, data, time.Now()); len(got) != 1 || got[ImmichSource].Value == nil {
		t.Errorf("a screen with only the Immich album gets %v", got)
	}
	if got := dataFor([]Dashboard{synologyPhotos}, data, time.Now()); len(got) != 1 || got[SynologySource].Value == nil {
		t.Errorf("a screen with only the Synology album gets %v", got)
	}
}
