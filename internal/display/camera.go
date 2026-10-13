package display

import (
	"time"
)

const (
	// CameraWidget is the type of the Camera widget. It has no data
	// source: the screen loads the image from the server itself, which
	// lets only screens of a display that shows the camera have it.
	CameraWidget = "camera"
	// SettingCamera is the ID of one of the cameras under Settings.
	SettingCamera = "camera"
	// DefaultCameraSeconds is the refresh of a Camera widget without one.
	DefaultCameraSeconds = 10
	// MinCameraSeconds keeps a camera from being asked more than every
	// few seconds; GDash shows stills, not video.
	MinCameraSeconds = 5
	maxCameraSeconds = 300
)

var cameraWidget = WidgetType{MinW: 4, MinH: 3, Settings: map[string]Setting{
	"camera":  {Kind: SettingCamera, Required: true},
	"seconds": {Kind: SettingInteger, Min: MinCameraSeconds, Max: maxCameraSeconds},
	// contain shows the whole image, cover fills the tile.
	"fit": {Kind: SettingChoice, Options: []string{"contain", "cover"}},
}}

// WidgetsAt returns the widgets the display shows at t: none outside its
// active hours, else those of its dashboard at t and of the pages it
// turns through. It decides which screens may load a camera's image.
func (v View) WidgetsAt(t time.Time) []Widget {
	if !v.Hours.Contains(t) {
		return nil
	}
	var widgets []Widget
	for _, d := range append([]Dashboard{v.DashboardAt(t)}, v.PagesAt(t)...) {
		widgets = append(widgets, d.Widgets...)
	}
	return widgets
}

// CameraRefresh returns how often the camera with id may be asked for an
// image for widgets: the shortest refresh of the Camera widgets among them
// that show it. ok is false when none does.
func CameraRefresh(widgets []Widget, id int64) (every time.Duration, ok bool) {
	for _, w := range widgets {
		if shown, isID := recordID(w.Settings["camera"]); w.Type != CameraWidget || !isID || shown != id {
			continue
		}
		seconds := DefaultCameraSeconds
		if n, isNumber := number(w.Settings["seconds"]); isNumber && n >= MinCameraSeconds {
			seconds = int(n)
		}
		if refresh := time.Duration(seconds) * time.Second; !ok || refresh < every {
			every = refresh
		}
		ok = true
	}
	return every, ok
}
