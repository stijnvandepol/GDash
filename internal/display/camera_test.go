package display

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/schedule"
)

func cameraBoard(id string, settings ...map[string]any) Dashboard {
	d := Dashboard{ID: id, Name: id, Theme: defaultTheme}
	for i, s := range settings {
		d.Widgets = append(d.Widgets, Widget{ID: "cam" + string(rune('a'+i)), Type: CameraWidget, X: 6 * i, Y: 0, W: 6, H: 4, Settings: s})
	}
	return Normalize(d)
}

func TestCameraWidgetSettingsAreChecked(t *testing.T) {
	cases := map[string]map[string]any{
		"":                               {"camera": 1.0, "seconds": 10.0, "fit": "cover"},
		"a camera widget needs a camera": {"seconds": 10.0},
		"camera must be the number":      {"camera": "front"},
		"seconds must be a whole number": {"camera": 1.0, "seconds": 2.0},
		"fit must be one of":             {"camera": 1.0, "fit": "stretch"},
	}
	for want, settings := range cases {
		err := ValidateDashboard(cameraBoard("cams", settings))
		switch {
		case want == "" && err != nil:
			t.Errorf("%v: %v", settings, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%v = %v, want %q", settings, err, want)
		}
	}
}

func TestCameraRefreshIsTheShortestOfTheWidgetsThatShowIt(t *testing.T) {
	widgets := cameraBoard("cams",
		map[string]any{"camera": 1.0, "seconds": 30.0},
		map[string]any{"camera": 1.0, "seconds": 15.0},
		map[string]any{"camera": 2.0},
	).Widgets
	if every, ok := CameraRefresh(widgets, 1); !ok || every != 15*time.Second {
		t.Errorf("camera 1 = %v %v, want 15s", every, ok)
	}
	if every, ok := CameraRefresh(widgets, 2); !ok || every != DefaultCameraSeconds*time.Second {
		t.Errorf("camera 2 without seconds = %v %v, want the default", every, ok)
	}
	if _, ok := CameraRefresh(widgets, 3); ok {
		t.Error("camera 3 is not shown, but has a refresh")
	}
}

func TestWidgetsAtFollowsTheDisplayAndItsHours(t *testing.T) {
	at := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	view := View{
		Dashboard: cameraBoard("hall", map[string]any{"camera": 1.0}),
		Pages:     []Dashboard{cameraBoard("garden", map[string]any{"camera": 2.0})},
	}
	if _, ok := CameraRefresh(view.WidgetsAt(at), 2); !ok {
		t.Error("a camera on a page the display turns to is not shown")
	}
	view.Hours = schedule.Hours{From: "22:00", Until: "23:00"}
	if widgets := view.WidgetsAt(at); len(widgets) != 0 {
		t.Errorf("outside active hours the display shows %d widgets", len(widgets))
	}
}

func TestOnlyScreensWithAnImmichWidgetGetTheAlbum(t *testing.T) {
	immichPhotos := Normalize(Dashboard{ID: "family", Widgets: []Widget{
		{ID: "album", Type: "photos", W: 6, H: 4, Settings: map[string]any{"source": PhotosImmich}},
	}})
	uploaded := Normalize(Dashboard{ID: "hall", Widgets: []Widget{{ID: "photos", Type: "photos", W: 6, H: 4}}})
	if immichPhotos.Widgets[0].Source != ImmichSource || uploaded.Widgets[0].Source != PhotosSource {
		t.Fatalf("sources = %q, %q", immichPhotos.Widgets[0].Source, uploaded.Widgets[0].Source)
	}
	if !UsesImmich([]Dashboard{uploaded, immichPhotos}) || UsesImmich([]Dashboard{uploaded}) {
		t.Fatal("UsesImmich does not follow the Photos widgets")
	}
	data := map[string]datahub.Entry{
		ImmichSource: {Value: json.RawMessage(`["4b1c5d6e-7f80-4912-a3b4-c5d6e7f80912"]`)},
		PhotosSource: {Value: json.RawMessage(`[]`)},
	}
	if got := dataFor([]Dashboard{uploaded}, data, time.Now()); len(got) != 1 || got[ImmichSource].Value != nil {
		t.Errorf("a screen without the album gets %v", got)
	}
	if got := dataFor([]Dashboard{immichPhotos}, data, time.Now()); got[ImmichSource].Value == nil || got[PhotosSource].Value != nil {
		t.Errorf("a screen with only the album gets %v", got)
	}
}
