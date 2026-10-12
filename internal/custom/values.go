package custom

import (
	"fmt"
	"strings"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// Limits of the named values of one source: enough for every value a
// household shows from one API, small enough to pick from a list.
const (
	MaxNamedValues = 40
	MaxNamedLists  = 10
	maxValueName   = 40
)

// Values are the values of a source that an admin named under Settings, so
// the dashboard editor offers them by name instead of by path. A widget
// still stores the path, so a renamed or removed value never breaks it.
type Values struct {
	Values []NamedValue `json:"values"`
	Lists  []NamedList  `json:"lists"`
}

// NamedValue is one value in the answer, such as "Outside" at
// current.temperature_2m, with how a widget shows it.
type NamedValue struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Unit string `json:"unit,omitempty"`
	// Decimals is how many decimals a number shows; nil shows it as sent.
	Decimals *int `json:"decimals,omitempty"`
}

// NamedList is a list in the answer with the values read from each item.
type NamedList struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Columns []Column `json:"columns"`
}

// Column is a value read from each item of a list; its path starts at the
// item, and $ is the item itself.
type Column struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// CheckValues validates named values and returns them with empty lists
// instead of nil. Names are unique per source, ignoring case; a column's
// name is unique within its list. It returns a *validation.Error whose
// fields name the value, such as values[2].path or lists[0].columns[1].name.
func CheckValues(in Values) (Values, error) {
	var errs validation.Collector
	out := Values{Values: []NamedValue{}, Lists: []NamedList{}}
	if len(in.Values) > MaxNamedValues {
		errs.Add("values", fmt.Sprintf("name at most %d values", MaxNamedValues))
	}
	if len(in.Lists) > MaxNamedLists {
		errs.Add("lists", fmt.Sprintf("name at most %d lists", MaxNamedLists))
	}
	if err := errs.Err(); err != nil {
		return Values{}, err
	}
	names := map[string]bool{}
	for i, v := range in.Values {
		field := fmt.Sprintf("values[%d]", i)
		checkValueName(&errs, field+".name", v.Name, names)
		checkPath(&errs, field+".path", v.Path)
		if !plainText(v.Unit, maxUnit) {
			errs.Add(field+".unit", fmt.Sprintf("use at most %d characters on one line", maxUnit))
		}
		if v.Decimals != nil && (*v.Decimals < 0 || *v.Decimals > MaxDecimals) {
			errs.Add(field+".decimals", fmt.Sprintf("use a whole number from 0 to %d", MaxDecimals))
		}
		out.Values = append(out.Values, v)
	}
	for i, l := range in.Lists {
		field := fmt.Sprintf("lists[%d]", i)
		checkValueName(&errs, field+".name", l.Name, names)
		checkPath(&errs, field+".path", l.Path)
		if len(l.Columns) == 0 || len(l.Columns) > MaxFields {
			errs.Add(field+".columns", fmt.Sprintf("choose 1 to %d values to show for each item", MaxFields))
		}
		columns := map[string]bool{}
		for j, c := range l.Columns {
			checkValueName(&errs, fmt.Sprintf("%s.columns[%d].name", field, j), c.Name, columns)
			checkPath(&errs, fmt.Sprintf("%s.columns[%d].path", field, j), c.Path)
		}
		if l.Columns == nil {
			l.Columns = []Column{}
		}
		out.Lists = append(out.Lists, l)
	}
	if err := errs.Err(); err != nil {
		return Values{}, err
	}
	return out, nil
}

// checkValueName checks a name and records it in seen, which holds the
// names taken so far in lower case.
func checkValueName(errs *validation.Collector, field, name string, seen map[string]bool) {
	if name == "" || !plainText(name, maxValueName) {
		errs.Add(field, fmt.Sprintf("use 1 to %d characters on one line", maxValueName))
		return
	}
	key := strings.ToLower(name)
	if seen[key] {
		errs.Add(field, "this name is already used; give each value its own name")
		return
	}
	seen[key] = true
}

func checkPath(errs *validation.Collector, field, text string) {
	if _, err := ParsePath(text); err != nil {
		errs.Add(field, err.Error())
	}
}
