package display

import (
	"fmt"
	"slices"

	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/datahub"
)

const (
	// NowPlayingSource is the data key of what plays on the household's
	// Cast devices: device ID to what it plays (nowplaying.Item), for the
	// devices that play or pause something.
	NowPlayingSource = "nowplaying.default"
	// NowPlayingWidget is the type of the Now playing widget.
	NowPlayingWidget = "nowplaying"
	// SettingCastDevices is a list of Cast device IDs, from discovery;
	// Max bounds how many. Empty means any device.
	SettingCastDevices = "castDevices"
	// MaxNowPlayingDevices keeps a widget's list of devices short enough
	// to choose from.
	MaxNowPlayingDevices = 8
)

// nowPlayingWidget shows what plays on the devices in devices, or on any
// device when the setting is absent or empty.
var nowPlayingWidget = WidgetType{Source: NowPlayingSource, MinW: 6, MinH: 4, Settings: map[string]Setting{
	"devices": {Kind: SettingCastDevices, Max: MaxNowPlayingDevices},
}}

// castDeviceIDs reads a devices setting: absent (nil) or a list of
// distinct device IDs, at most MaxNowPlayingDevices. ok is false for
// anything else.
func castDeviceIDs(value any) ([]string, bool) {
	if value == nil {
		return nil, true
	}
	return idList(value, MaxNowPlayingDevices, cast.ValidDeviceID)
}

func castDevicesProblem(key string, value any) string {
	if _, ok := castDeviceIDs(value); !ok {
		return fmt.Sprintf("%s must be at most %d Cast device IDs from discovery, or empty for any device", key, MaxNowPlayingDevices)
	}
	return ""
}

// NowPlayingDevices returns the Cast devices that Now playing widgets
// follow, sorted and without repeats, and whether one of the widgets
// follows any device.
func NowPlayingDevices(dashboards []Dashboard) (ids []string, anyDevice bool) {
	for _, d := range dashboards {
		for _, w := range d.Widgets {
			if w.Type != NowPlayingWidget {
				continue
			}
			chosen, ok := castDeviceIDs(w.Settings["devices"])
			if !ok {
				continue
			}
			if len(chosen) == 0 {
				anyDevice = true
			}
			ids = append(ids, chosen...)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), anyDevice
}

// onlyNowPlaying narrows the Now playing entry to the devices that the
// widgets of one screen follow: a screen with a widget for the kitchen
// speaker does not learn what plays in the bedroom.
func onlyNowPlaying(entry datahub.Entry, dashboards []Dashboard) datahub.Entry {
	ids, anyDevice := NowPlayingDevices(dashboards)
	if anyDevice {
		return entry
	}
	return onlyKeys(entry, ids)
}
