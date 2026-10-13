package display

import (
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// Every dashboard uses the same 16:9 grid, so a layout looks the same on any
// TV and the editor can show it as the TV does.
const (
	GridColumns = 24
	GridRows    = 14

	// DefaultID is the dashboard of displays that have none assigned.
	DefaultID = "default"

	maxWidgets    = 30
	maxNameLength = 60
	defaultTheme  = "minimal-dark"
	dateLayout    = "2006-01-02"
)

// Kinds of widget setting.
const (
	SettingInteger = "integer"
	SettingText    = "text"
	SettingDate    = "date"
	// SettingEntities is a list of Home Assistant entity IDs; Min and Max
	// bound how many.
	SettingEntities = "entities"
	// SettingStop is the code of a public transport stop area, chosen with
	// the stop search.
	SettingStop = "stop"
	// SettingPlace is a place chosen with the place search: an object with
	// a name, a latitude and a longitude.
	SettingPlace = "place"
	// SettingChoice is one of Options.
	SettingChoice = "choice"
	// SettingBoolean is true or false.
	SettingBoolean = "boolean"
)

// Setting is one setting a widget type takes.
type Setting struct {
	Kind string `json:"kind"`
	// Min and Max bound an integer, or the length of a text in characters.
	Min      int  `json:"min,omitempty"`
	Max      int  `json:"max,omitempty"`
	Required bool `json:"required"`
	// Options are the values a choice takes.
	Options []string `json:"options,omitempty"`
}

// WidgetType describes a kind of widget: where its data comes from, how
// small it may be and what settings it takes.
type WidgetType struct {
	// Source is the data hub key the widget shows, or empty for widgets
	// that need no data, such as the clock.
	Source   string
	MinW     int
	MinH     int
	Settings map[string]Setting
}

var maxItems = map[string]Setting{"maxItems": {Kind: SettingInteger, Min: 1, Max: 20}}

// WidgetTypes is the catalogue of widgets. A widget's data source follows
// from its type, so a dashboard cannot point a widget at arbitrary data.
var WidgetTypes = map[string]WidgetType{
	"clock":       {MinW: 4, MinH: 3},
	"weather":     {Source: WeatherSource, MinW: 4, MinH: 4},
	"forecast":    {Source: WeatherSource, MinW: 6, MinH: 4, Settings: map[string]Setting{"days": {Kind: SettingInteger, Min: 1, Max: 7}}},
	"sun":         {Source: WeatherSource, MinW: 5, MinH: 4},
	"moon":        {MinW: 4, MinH: 4},
	"airquality":  {Source: AirQualitySource, MinW: 5, MinH: 4},
	"waste":       {Source: WasteSource, MinW: 6, MinH: 4, Settings: wasteSettings},
	"energyprice": {Source: EnergyPriceSource, MinW: 6, MinH: 4},
	"solar":       {Source: SolarSource, MinW: 6, MinH: 4},
	"news":        {Source: "news.default", MinW: 6, MinH: 4, Settings: maxItems},
	"calendar":    {Source: "calendar.default", MinW: 6, MinH: 4, Settings: maxItems},
	"text":        {MinW: 4, MinH: 3, Settings: map[string]Setting{"text": {Kind: SettingText, Min: 1, Max: 300, Required: true}}},
	"photos": {Source: PhotosSource, MinW: 4, MinH: 3, Settings: map[string]Setting{
		"seconds": {Kind: SettingInteger, Min: 5, Max: 600},
		"source":  {Kind: SettingChoice, Options: []string{PhotosUploaded, PhotosImmich, PhotosSynology}},
	}},
	"countdown": {MinW: 4, MinH: 4, Settings: map[string]Setting{
		"label": {Kind: SettingText, Min: 1, Max: 60, Required: true},
		"date":  {Kind: SettingDate, Required: true},
	}},
	"homeassistant": {Source: HomeAssistantSource, MinW: 4, MinH: 3, Settings: map[string]Setting{
		"entities": {Kind: SettingEntities, Min: 1, Max: MaxEntitiesPerWidget, Required: true},
	}},
	"holidays":          holidaysWidget,
	"departures":        departuresWidget,
	"fuel":              fuelWidget,
	"rain":              rainWidget,
	"warnings":          warningsWidget,
	"homey":             homeyWidget,
	HomeyPresenceWidget: homeyPresenceWidget,
	CustomWidget:        customWidget,
	CameraWidget:        cameraWidget,
	NowPlayingWidget:    nowPlayingWidget,
}

// AirQualitySource is the data key of the air quality and pollen at the
// weather location.
const AirQualitySource = "airquality.current"

// HomeAssistantSource is the data key of the entity states that Home
// Assistant widgets show.
const HomeAssistantSource = "homeassistant.default"

// MaxEntitiesPerWidget keeps a Home Assistant widget readable from across
// a room.
const MaxEntitiesPerWidget = 8

// PhotosSource is the data key of the photo list; the server builds it
// from the photo folder rather than fetching it.
const PhotosSource = "photos.default"

// Where a Photos widget takes its photos from: the uploaded ones, the
// default, or the album of Immich or Synology Photos chosen under Integrations.
const (
	PhotosUploaded = "uploaded"
	PhotosImmich   = "immich"
	PhotosSynology = "synology"
)

// The data keys of the albums: a random part of each album's photos, by
// IDs that screens load through the server.
const (
	ImmichSource   = "photos.immich"
	SynologySource = "photos.synology"
)

// albumSources maps a Photos widget's source to its data key.
var albumSources = map[string]string{PhotosImmich: ImmichSource, PhotosSynology: SynologySource}

// widgetSource is the data key a widget shows: its type's, except for a
// Photos widget that shows an album.
func widgetSource(w Widget) string {
	if source, ok := w.Settings["source"].(string); ok && w.Type == "photos" && albumSources[source] != "" {
		return albumSources[source]
	}
	return WidgetTypes[w.Type].Source
}

// UsesAlbum reports whether a Photos widget on the dashboards shows the
// album with data key, so an album is only read while a screen can show it.
func UsesAlbum(dashboards []Dashboard, key string) bool {
	return slices.ContainsFunc(dashboards, func(d Dashboard) bool {
		return slices.ContainsFunc(d.Widgets, func(w Widget) bool { return ShowsAlbum(w, key) })
	})
}

// ShowsAlbum reports whether w is a Photos widget that shows the album
// with data key.
func ShowsAlbum(w Widget, key string) bool { return w.Type == "photos" && widgetSource(w) == key }

var validDashboardID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

var validWidgetID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// ValidDashboardID reports whether id can name a dashboard.
func ValidDashboardID(id string) bool { return validDashboardID.MatchString(id) }

// Normalize fills in what a client does not decide: the grid size and each
// widget's data source from its type, whatever the client sent. An empty
// theme becomes the default; an unknown one is left for ValidateDashboard
// to reject.
func Normalize(d Dashboard) Dashboard {
	d.Columns, d.Rows = GridColumns, GridRows
	if d.Theme == "" {
		d.Theme = defaultTheme
	}
	widgets := make([]Widget, len(d.Widgets))
	for i, w := range d.Widgets {
		w.Source = widgetSource(w)
		widgets[i] = w
	}
	d.Widgets = widgets
	return d
}

// ValidateDashboard checks a dashboard before it is stored. It returns a
// *validation.Error listing every problem, with widget problems as
// "widgets[i]".
func ValidateDashboard(d Dashboard) error {
	var errs validation.Collector
	if !validDashboardID.MatchString(d.ID) {
		errs.Add("id", "use 1 to 32 lowercase letters, digits or dashes, starting with a letter or digit")
	}
	name := strings.TrimSpace(d.Name)
	if name == "" || name != d.Name || utf8.RuneCountInString(name) > maxNameLength {
		errs.Add("name", "use 1 to 60 characters without leading or trailing spaces")
	}
	if !slices.Contains(Themes, d.Theme) {
		errs.Add("theme", "choose one of: "+strings.Join(Themes, ", "))
	}
	if d.NightTheme != "" && !slices.Contains(Themes, d.NightTheme) {
		errs.Add("nightTheme", "choose one of: "+strings.Join(Themes, ", ")+", or none to keep the theme")
	}
	if d.Font != "" && !slices.Contains(Fonts, d.Font) {
		errs.Add("font", "choose one of: "+strings.Join(Fonts, ", ")+", or none for the theme's own")
	}
	if len(d.Widgets) > maxWidgets {
		errs.Add("widgets", fmt.Sprintf("use at most %d widgets", maxWidgets))
		return errs.Err()
	}
	seen := map[string]bool{}
	var cells [GridColumns][GridRows]bool
	for i, w := range d.Widgets {
		field := fmt.Sprintf("widgets[%d]", i)
		if msg := widgetProblem(w, seen); msg != "" {
			errs.Add(field, msg)
			continue
		}
		if overlaps(&cells, w) {
			errs.Add(field, "overlaps another widget")
		}
	}
	return errs.Err()
}

// widgetProblem returns why a widget is invalid on its own, or "".
func widgetProblem(w Widget, seen map[string]bool) string {
	kind, known := WidgetTypes[w.Type]
	switch {
	case !validWidgetID.MatchString(w.ID):
		return "id must be 1 to 32 lowercase letters, digits or dashes"
	case seen[w.ID]:
		return "id is used by another widget"
	case !known:
		return fmt.Sprintf("unknown widget type %q", w.Type)
	// Each bound on its own first: a huge X plus W would wrap around.
	case w.X < 0 || w.Y < 0 || w.X > GridColumns || w.Y > GridRows || w.W > GridColumns || w.H > GridRows ||
		w.X+w.W > GridColumns || w.Y+w.H > GridRows:
		return fmt.Sprintf("must fit the %dx%d grid", GridColumns, GridRows)
	case w.W < kind.MinW || w.H < kind.MinH:
		return fmt.Sprintf("a %s widget needs at least %dx%d cells", w.Type, kind.MinW, kind.MinH)
	}
	seen[w.ID] = true
	if w.Type == CustomWidget {
		return customProblem(w.Settings)
	}
	return settingsProblem(w, kind)
}

// settingsProblem returns the first problem with a widget's settings, in
// the order of their names, so the same layout always gets the same error.
func settingsProblem(w Widget, kind WidgetType) string {
	for _, key := range slices.Sorted(maps.Keys(w.Settings)) {
		rule, known := kind.Settings[key]
		if !known {
			return fmt.Sprintf("unknown setting %q", key)
		}
		if msg := valueProblem(key, w.Settings[key], rule); msg != "" {
			return msg
		}
	}
	for _, key := range slices.Sorted(maps.Keys(kind.Settings)) {
		rule := kind.Settings[key]
		if _, set := w.Settings[key]; !rule.Required || set {
			continue
		}
		// These kinds have a message that says how to choose one.
		if rule.Kind == SettingEntities || rule.Kind == SettingStop || rule.Kind == SettingHomeyCapabilities || rule.Kind == SettingHomeyUsers {
			return valueProblem(key, nil, rule)
		}
		return fmt.Sprintf("a %s widget needs a %s", w.Type, key)
	}
	return ""
}

func valueProblem(key string, value any, rule Setting) string {
	switch rule.Kind {
	case SettingInteger:
		n, ok := number(value)
		if !ok || n != math.Trunc(n) || n < float64(rule.Min) || n > float64(rule.Max) {
			return fmt.Sprintf("%s must be a whole number from %d to %d", key, rule.Min, rule.Max)
		}
	case SettingText:
		text, ok := value.(string)
		n := utf8.RuneCountInString(text)
		if !ok || n < rule.Min || n > rule.Max || strings.TrimSpace(text) == "" || hasControl(text) {
			return fmt.Sprintf("%s must be %d to %d characters of text", key, rule.Min, rule.Max)
		}
	case SettingDate:
		text, ok := value.(string)
		if _, err := time.Parse(dateLayout, text); !ok || err != nil {
			return fmt.Sprintf("%s must be a date as YYYY-MM-DD", key)
		}
	case SettingCalendar:
		if _, ok := recordID(value); !ok {
			return fmt.Sprintf("%s must be the number of a calendar added under Integrations", key)
		}
	case SettingCamera:
		if _, ok := recordID(value); !ok {
			return fmt.Sprintf("%s must be the number of a camera added under Integrations", key)
		}
	case SettingEntities:
		if ids, ok := entityIDs(value, rule.Max); !ok || len(ids) < rule.Min {
			return fmt.Sprintf("choose %d to %d Home Assistant entities", rule.Min, rule.Max)
		}
	case SettingHomeyCapabilities, SettingHomeyUsers:
		return homeySettingProblem(key, value, rule)
	case SettingStop:
		return stopProblem(key, value)
	case SettingPlace:
		return placeProblem(key, value)
	case SettingChoice:
		return choiceProblem(key, value, rule)
	case SettingBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("%s must be true or false", key)
		}
	case SettingCastDevices:
		return castDevicesProblem(key, value)
	}
	return ""
}

// hasControl reports control characters other than a line break, which a
// note on the screen may use.
func hasControl(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r != '\n' && unicode.IsControl(r) })
}

func overlaps(cells *[GridColumns][GridRows]bool, w Widget) bool {
	hit := false
	for x := w.X; x < w.X+w.W; x++ {
		for y := w.Y; y < w.Y+w.H; y++ {
			if cells[x][y] {
				hit = true
			}
			cells[x][y] = true
		}
	}
	return hit
}
