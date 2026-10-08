package display

import (
	"fmt"
	"regexp"
	"strings"
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

	maxWidgets       = 30
	maxNameLength    = 60
	defaultTheme     = "minimal-dark"
	minMaxItems      = 1
	maxMaxItemsLimit = 20
)

// WidgetType describes a kind of widget: where its data comes from and how
// small it may be.
type WidgetType struct {
	// Source is the data hub key the widget shows, or empty for widgets
	// that need no data, such as the clock.
	Source string
	MinW   int
	MinH   int
	// HasMaxItems marks widgets that show a list and take a "maxItems"
	// setting.
	HasMaxItems bool
}

// WidgetTypes is the catalogue of widgets. A widget's data source follows
// from its type, so a dashboard cannot point a widget at arbitrary data.
var WidgetTypes = map[string]WidgetType{
	"clock":    {MinW: 4, MinH: 3},
	"weather":  {Source: "weather.current", MinW: 4, MinH: 4},
	"news":     {Source: "news.default", MinW: 6, MinH: 3, HasMaxItems: true},
	"calendar": {Source: "calendar.default", MinW: 6, MinH: 4, HasMaxItems: true},
}

var validDashboardID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

var validWidgetID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// ValidDashboardID reports whether id can name a dashboard.
func ValidDashboardID(id string) bool { return validDashboardID.MatchString(id) }

// Normalize fills in what a client does not decide: the grid size, the
// theme, and each widget's data source from its type. Client values for
// these are ignored.
func Normalize(d Dashboard) Dashboard {
	d.Columns, d.Rows, d.Theme = GridColumns, GridRows, defaultTheme
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

func settingsProblem(w Widget, kind WidgetType) string {
	for key, value := range w.Settings {
		if key != "maxItems" || !kind.HasMaxItems {
			return fmt.Sprintf("unknown setting %q", key)
		}
		// JSON numbers decode as float64.
		n, ok := value.(float64)
		if !ok {
			if i, isInt := value.(int); isInt {
				n, ok = float64(i), true
			}
		}
		if !ok || n != float64(int(n)) || n < minMaxItems || n > maxMaxItemsLimit {
			return fmt.Sprintf("maxItems must be a whole number from %d to %d", minMaxItems, maxMaxItemsLimit)
		}
	}
	return ""
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
