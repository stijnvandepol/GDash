package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

const (
	kitchenSpeaker = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bedroomSpeaker = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// nowPlayingAt returns a Now playing widget below the default layout.
func nowPlayingAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: NowPlayingWidget, X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func TestNowPlayingSettingsAreValidated(t *testing.T) {
	tooMany := make([]any, MaxNowPlayingDevices+1)
	for i := range tooMany {
		tooMany[i] = strings.Repeat(string(rune('a'+i)), 32)
	}
	const problem = "widgets[2]: devices must be at most 8 Cast device IDs from discovery, or empty for any device"
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"any device", nil, ""},
		{"any device, said with an empty list", map[string]any{"devices": []any{}}, ""},
		{"two speakers", map[string]any{"devices": []any{kitchenSpeaker, bedroomSpeaker}}, ""},
		{"a device twice", map[string]any{"devices": []any{kitchenSpeaker, kitchenSpeaker}}, problem},
		{"a name instead of an ID", map[string]any{"devices": []any{"Kitchen speaker"}}, problem},
		{"an upper-case ID", map[string]any{"devices": []any{strings.ToUpper(kitchenSpeaker)}}, problem},
		{"one ID as text", map[string]any{"devices": kitchenSpeaker}, problem},
		{"too many", map[string]any{"devices": tooMany}, problem},
		{"unknown setting", map[string]any{"maxItems": float64(3)}, `widgets[2]: unknown setting "maxItems"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, nowPlayingAt("music", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
	small := valid()
	small.Widgets = append(small.Widgets, Widget{ID: "music", Type: NowPlayingWidget, X: 0, Y: 5, W: 5, H: 4})
	if got := fields(ValidateDashboard(small)); !strings.Contains(got, "needs at least 6x4 cells") {
		t.Fatalf("a 5x4 widget: errors = %q", got)
	}
}

func TestNowPlayingDevicesAreCollectedOnce(t *testing.T) {
	a := Dashboard{Widgets: []Widget{
		nowPlayingAt("one", map[string]any{"devices": []any{bedroomSpeaker, kitchenSpeaker}}),
		{Type: "text", Settings: map[string]any{"devices": []any{"ignored"}}},
	}}
	b := Dashboard{Widgets: []Widget{nowPlayingAt("two", map[string]any{"devices": []any{kitchenSpeaker}})}}
	ids, anyDevice := NowPlayingDevices([]Dashboard{a, b})
	if got := strings.Join(ids, ","); got != kitchenSpeaker+","+bedroomSpeaker || anyDevice {
		t.Fatalf("NowPlayingDevices = %s, %v; want both speakers once and not any", got, anyDevice)
	}
	if _, anyDevice := NowPlayingDevices([]Dashboard{a, {Widgets: []Widget{nowPlayingAt("all", nil)}}}); !anyDevice {
		t.Fatal("a widget without devices does not follow any device")
	}
	if ids, anyDevice := NowPlayingDevices([]Dashboard{valid()}); ids != nil || anyDevice {
		t.Fatalf("without the widget: %v, %v", ids, anyDevice)
	}
}

func TestEachScreenLearnsOnlyWhatItsWidgetsFollow(t *testing.T) {
	kitchen := Normalize(Dashboard{ID: "kitchen", Widgets: []Widget{nowPlayingAt("music", map[string]any{"devices": []any{kitchenSpeaker}})}})
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{nowPlayingAt("music", nil)}})
	office := Normalize(Dashboard{ID: "office", Widgets: []Widget{{ID: "clock", Type: "clock", W: 4, H: 3}}})
	all := `{"` + bedroomSpeaker + `":{"title":"Lullaby"},"` + kitchenSpeaker + `":{"title":"Cooking"}}`
	data := map[string]datahub.Entry{NowPlayingSource: {Value: []byte(all)}}
	b := NewBroadcaster()
	views := map[string]View{"kitchen": {Dashboard: kitchen}, "hall": {Dashboard: hall}, "office": {Dashboard: office}}
	if err := b.Publish(now, views, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if got := string(decode(t, <-b.Subscribe(ctx, "kitchen")).Data[NowPlayingSource].Value); got != `{"`+kitchenSpeaker+`":{"title":"Cooking"}}` {
		t.Fatalf("kitchen received %s, want only the kitchen speaker", got)
	}
	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[NowPlayingSource].Value); got != all {
		t.Fatalf("hall, which follows any device, received %s", got)
	}
	if _, ok := decode(t, <-b.Subscribe(ctx, "office")).Data[NowPlayingSource]; ok {
		t.Fatal("a screen without the widget learned what plays")
	}

	// What a screen was sent decides which covers it may load.
	if entry, ok := b.Sent("kitchen", NowPlayingSource); !ok || strings.Contains(string(entry.Value), "Lullaby") {
		t.Fatalf("Sent(kitchen) = %s, %v", entry.Value, ok)
	}
	if _, ok := b.Sent("office", NowPlayingSource); ok {
		t.Fatal("Sent(office) has the Now playing entry")
	}
}
