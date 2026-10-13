package cast

import (
	"os"
	"strings"
	"testing"
)

// The fixtures in testdata follow MEDIA_STATUS messages as Spotify, the
// YouTube app, the Default Media Receiver and a radio app send them (shapes
// from pychromecast issue logs and Google's Cast reference); the IDs in
// them are made up.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplyMediaStatusReadsRealApps(t *testing.T) {
	tests := []struct {
		fixture string
		want    Media
	}{
		{"media-spotify.json", Media{
			Title: "One More Time", Artist: "Daft Punk", Album: "Discovery",
			Image: "https://i.scdn.co/image/ab67616d0000b2738a5a2a9f6f2c1e5d3b4c7e10",
			State: PlayerPlaying, Position: 63.514, Duration: 320.357,
		}},
		{"media-youtube.json", Media{
			Title:  "Big Buck Bunny 60fps 4K - Official Blender Foundation Short Film",
			Artist: "Blender", Image: "https://i.ytimg.com/vi/aqz-KE-bpKQ/hqdefault.jpg",
			State: PlayerPaused, Position: 125.77, Duration: 634.567,
		}},
		{"media-default-receiver.json", Media{
			Title: "Big Buck Bunny", Artist: "By Blender Foundation",
			Image: "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/images/BigBuckBunny.jpg",
			State: PlayerBuffering, Duration: 596.474195,
		}},
		// A live stream has no duration, and its data: image is not fetched.
		{"media-radio.json", Media{Title: "NPO Radio 2", State: PlayerPlaying, Position: 1843.2}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got, ok := applyMediaStatus(readFixture(t, tt.fixture), Media{})
			if !ok {
				t.Fatal("not read as a MEDIA_STATUS")
			}
			if got != tt.want {
				t.Fatalf("got  %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestApplyMediaStatusKeepsWhatAnUpdateLeavesOut(t *testing.T) {
	playing, _ := applyMediaStatus(readFixture(t, "media-spotify.json"), Media{})
	// Apps send "media" only when it changed: a pause carries the state
	// and the time.
	paused, ok := applyMediaStatus(`{"type":"MEDIA_STATUS","status":[{"mediaSessionId":1,"playerState":"PAUSED","currentTime":70.1}],"requestId":0}`, playing)
	if !ok {
		t.Fatal("update not read")
	}
	want := playing
	want.State, want.Position = PlayerPaused, 70.1
	if paused != want {
		t.Fatalf("got  %+v\nwant %+v", paused, want)
	}
}

func TestApplyMediaStatusEmptyListMeansNothingPlays(t *testing.T) {
	playing, _ := applyMediaStatus(readFixture(t, "media-spotify.json"), Media{})
	got, ok := applyMediaStatus(`{"type":"MEDIA_STATUS","status":[],"requestId":3}`, playing)
	if !ok || got != (Media{}) {
		t.Fatalf("got %+v, %v; want nothing", got, ok)
	}
}

func TestApplyMediaStatusIgnoresOtherMessages(t *testing.T) {
	current := Media{Title: "Kept"}
	for _, payload := range []string{
		`{"type":"LOAD_FAILED","requestId":1}`,
		`{"type":"MEDIA_STATUS","status":`,
		`not json`,
	} {
		if got, ok := applyMediaStatus(payload, current); ok || got != current {
			t.Fatalf("%s: got %+v, %v", payload, got, ok)
		}
	}
}

func TestApplyMediaStatusBoundsWhatAnAppSends(t *testing.T) {
	long := strings.Repeat("é", maxMediaText+50)
	payload := `{"type":"MEDIA_STATUS","status":[{"playerState":"PLAYING","currentTime":-4,"media":{"duration":-1,"metadata":{` +
		`"title":"  ` + long + `  ","images":[{"url":"javascript:alert(1)"}]}}}]}`
	got, ok := applyMediaStatus(payload, Media{})
	if !ok {
		t.Fatal("not read")
	}
	if n := len([]rune(got.Title)); n != maxMediaText {
		t.Fatalf("title has %d characters, want %d", n, maxMediaText)
	}
	if got.Image != "" || got.Position != 0 || got.Duration != 0 {
		t.Fatalf("got image %q, position %v, duration %v; want none of them", got.Image, got.Position, got.Duration)
	}
	tooLong := `{"type":"MEDIA_STATUS","status":[{"media":{"metadata":{"images":[{"url":"https://example.com/` + strings.Repeat("a", maxImageURL) + `"}]}}}]}`
	if got, _ := applyMediaStatus(tooLong, Media{}); got.Image != "" {
		t.Fatal("an overlong image address was kept")
	}
}

func TestMetadataNamesTheArtistBySeriesOrSubtitle(t *testing.T) {
	tests := []struct {
		meta mediaMetadata
		want string
	}{
		{mediaMetadata{Artist: "Artist", AlbumArtist: "Album artist", Subtitle: "Sub"}, "Artist"},
		{mediaMetadata{AlbumArtist: "Album artist", Subtitle: "Sub"}, "Album artist"},
		{mediaMetadata{SeriesTitle: "The Series", Subtitle: "Episode summary"}, "The Series"},
		{mediaMetadata{Subtitle: "Channel"}, "Channel"},
		{mediaMetadata{Artist: "  ", Subtitle: "Channel"}, "Channel"},
	}
	for _, tt := range tests {
		if _, artist, _, _ := tt.meta.describe(); artist != tt.want {
			t.Errorf("%+v: artist %q, want %q", tt.meta, artist, tt.want)
		}
	}
}
