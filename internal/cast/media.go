package cast

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"
)

// nsMedia is the namespace of the Cast media protocol. Every media app
// speaks it: Spotify, YouTube Music, Plex and the Default Media Receiver.
const nsMedia = "urn:x-cast:com.google.cast.media"

// Player states of the media protocol.
const (
	PlayerPlaying   = "PLAYING"
	PlayerPaused    = "PAUSED"
	PlayerBuffering = "BUFFERING"
	PlayerIdle      = "IDLE"
)

const (
	// maxMediaText bounds a title, artist or album: enough for any real
	// one, and it keeps a snapshot far below the 64 KiB Cast limit.
	maxMediaText = 200
	// maxImageURL bounds the cover art address; a longer one is dropped.
	maxImageURL = 2048
)

// Media is what a media app plays, as its MEDIA_STATUS says.
type Media struct {
	Title  string
	Artist string
	Album  string
	// Image is the address of the cover art, or "" when the app gives
	// none. It points anywhere the app likes and is never fetched here.
	Image string
	// State is one of the Player states, or "" while the app has no media.
	State string
	// Position and Duration are in seconds. Duration is 0 for a live
	// stream, or when the app does not say.
	Position float64
	Duration float64
}

type mediaStatusMessage struct {
	Type   string        `json:"type"`
	Status []mediaStatus `json:"status"`
}

type mediaStatus struct {
	PlayerState string   `json:"playerState"`
	CurrentTime *float64 `json:"currentTime"`
	Media       *struct {
		Duration *float64       `json:"duration"`
		Metadata *mediaMetadata `json:"metadata"`
	} `json:"media"`
}

// mediaMetadata holds the fields of the metadata types that name what
// plays: generic (YouTube), movie, TV show and music track.
type mediaMetadata struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Artist      string `json:"artist"`
	AlbumArtist string `json:"albumArtist"`
	AlbumName   string `json:"albumName"`
	SeriesTitle string `json:"seriesTitle"`
	Images      []struct {
		URL string `json:"url"`
	} `json:"images"`
}

// applyMediaStatus updates current with a MEDIA_STATUS payload. It reports
// false for any other message, which leaves current as it was.
//
// It reads a status the way pychromecast's MediaStatus.update does
// (pychromecast/controllers/media.py): only the first status counts, and a
// field the update leaves out keeps its last value, because apps send
// "media" only when it changed. An empty status list means the app has no
// media loaded, so nothing plays.
func applyMediaStatus(payload string, current Media) (Media, bool) {
	var msg mediaStatusMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil || msg.Type != "MEDIA_STATUS" {
		return current, false
	}
	if len(msg.Status) == 0 {
		return Media{}, true
	}
	status := msg.Status[0]
	if status.PlayerState != "" {
		current.State = status.PlayerState
	}
	if status.CurrentTime != nil {
		current.Position = max(0, *status.CurrentTime)
	}
	if media := status.Media; media != nil {
		if media.Duration != nil {
			current.Duration = max(0, *media.Duration)
		}
		if meta := media.Metadata; meta != nil {
			current.Title, current.Artist, current.Album, current.Image = meta.describe()
		}
	}
	return current, true
}

// describe returns the title, artist, album and cover art of what plays.
// The line under the title is the artist of a track, the series of a TV
// episode, or the subtitle that generic media such as a YouTube video use
// for the channel. The cover is the first image, as in Home Assistant's
// Cast integration: apps list the main image first.
func (m mediaMetadata) describe() (title, artist, album, image string) {
	artist = firstNonEmpty(m.Artist, m.AlbumArtist, m.SeriesTitle, m.Subtitle)
	if len(m.Images) > 0 {
		image = imageAddress(m.Images[0].URL)
	}
	return clip(m.Title), clip(artist), clip(m.AlbumName), image
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// clip trims a text and cuts it to maxMediaText characters.
func clip(s string) string {
	s = strings.TrimSpace(strings.ToValidUTF8(s, ""))
	if utf8.RuneCountInString(s) <= maxMediaText {
		return s
	}
	return string([]rune(s)[:maxMediaText])
}

// imageAddress returns raw when it is an absolute http or https address of
// reasonable length, or "".
func imageAddress(raw string) string {
	if len(raw) > maxImageURL {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
}

// askMediaStatus asks the media app at transportID for its status. The
// answer, a MEDIA_STATUS, arrives through AppMessages like the updates the
// app sends on its own.
func (c *Client) askMediaStatus(transportID string) error {
	return c.sendJSON(transportID, nsMedia, map[string]any{"type": "GET_STATUS", "requestId": c.requestID.Add(1)})
}
