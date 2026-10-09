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

	"github.com/stijnvandepol/daycast/internal/validation"
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
)

// Setting is one setting a widget type takes.
type Setting struct {
	Kind string `json:"kind"`
	// Min and Max bound an integer, or the length of a text in characters.
	Min      int  `json:"min,omitempty"`
	Max      int  `json:"max,omitempty"`
	Required bool `json:"required"`
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
	"clock":    {MinW: 4, MinH: 3},
	"weather":  {Source: "weather.current", MinW: 4, MinH: 4},
	"forecast": {Source: "weather.current", MinW: 6, MinH: 4, Settings: map[string]Setting{"days": {Kind: SettingInteger, Min: 1, Max: 7}}},
	"news":     {Source: "news.default", MinW: 6, MinH: 4, Settings: maxItems},
	"calendar": {Source: "calendar.default", MinW: 6, MinH: 4, Settings: maxItems},
	"text":     {MinW: 4, MinH: 3, Settings: map[string]Setting{"text": {Kind: SettingText, Min: 1, Max: 300, Required: true}}},
	"photos":   {Source: PhotosSource, MinW: 4, MinH: 3, Settings: map[string]Setting{"seconds": {Kind: SettingInteger, Min: 5, Max: 600}}},
	"countdown": {MinW: 4, MinH: 4, Settings: map[string]Setting{
		"label": {Kind: SettingText, Min: 1, Max: 60, Required: true},
		"date":  {Kind: SettingDate, Required: true},
	}},
}

// PhotosSource is the data key of the photo list; the server builds it
// from the photo folder rather than fetching it.
const PhotosSource = "photos.default"

// Themes are the built-in themes; web/src/theme.css defines their tokens.
var Themes = []string{"minimal-dark", "minimal-light", "glass", "warm-light", "high-contrast", "night"}

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
		w.Source = WidgetTypes[w.Type].Source
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
	case w.X < 0 || w.Y < 0 || w.X+w.W > GridColumns || w.Y+w.H > GridRows:
		return fmt.Sprintf("must fit the %dx%d grid", GridColumns, GridRows)
	case w.W < kind.MinW || w.H < kind.MinH:
		return fmt.Sprintf("a %s widget needs at least %dx%d cells", w.Type, kind.MinW, kind.MinH)
	}
	seen[w.ID] = true
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
		if _, set := w.Settings[key]; kind.Settings[key].Required && !set {
			return fmt.Sprintf("a %s widget needs a %s", w.Type, key)
		}
	}
	return ""
}

func valueProblem(key string, value any, rule Setting) string {
	switch rule.Kind {
	case SettingInteger:
		// JSON numbers decode as float64.
		n, ok := value.(float64)
		if i, isInt := value.(int); isInt {
			n, ok = float64(i), true
		}
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
