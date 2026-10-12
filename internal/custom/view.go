package custom

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Limits of what one widget sends a TV. A snapshot must stay far below the
// 64 KB of a Cast message, with room for other widgets.
const (
	maxRows      = 15
	maxCellRunes = 100
)

// States of a widget's data.
const (
	StateOK      = "ok"
	StateLoading = "loading"
	StateFailing = "failing"
	// StateMissing means the widget names a source that does not exist,
	// such as after importing a dashboard into another GDash.
	StateMissing = "missing"
)

// Data is a source's latest answer, for one widget.
type Data struct {
	// Doc is the answer, decoded; only meaningful when HasValue is true.
	Doc      any
	HasValue bool
	// Failing is true while the last refresh failed.
	Failing bool
	Stale   bool
	Missing bool
}

// Shown is what a TV receives for one Custom widget: plain values and text,
// never markup, and only the values the widget shows. The TV formats
// numbers in its language.
type Shown struct {
	State    string `json:"state"`
	Stale    bool   `json:"stale,omitempty"`
	Failing  bool   `json:"failing,omitempty"`
	Mode     string `json:"mode"`
	Title    string `json:"title,omitempty"`
	Label    string `json:"label,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Decimals *int   `json:"decimals,omitempty"`
	// Value is a number, text, true, false or null, in value mode.
	Value any         `json:"value"`
	Rows  []ShownRow  `json:"rows,omitempty"`
	Pairs []ShownPair `json:"pairs,omitempty"`
	// Badges are the labels of the badges that are on, in badges mode.
	// Those that are off are left out, so a TV never learns about them.
	Badges []string `json:"badges,omitempty"`
	// Tone applies to the value, or to every row and pair without its own.
	Tone    string `json:"tone,omitempty"`
	Message string `json:"message,omitempty"`
	// Hidden hides the data; the title and message still show.
	Hidden bool `json:"hidden,omitempty"`
}

// ShownRow is one item of a list.
type ShownRow struct {
	Cells []any  `json:"cells"`
	Tone  string `json:"tone,omitempty"`
}

// ShownPair is one line of key-values mode.
type ShownPair struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// Show builds what the widget shows for d at now.
func (w Widget) Show(d Data, now time.Time) Shown {
	shown, _ := w.evaluate(d, now)
	return shown
}

// Matching reports, per rule, whether it holds for doc at now. A rule for
// rows holds when it holds for any row.
func (w Widget) Matching(doc any, now time.Time) []bool {
	_, matched := w.evaluate(Data{Doc: doc, HasValue: true}, now)
	return matched
}

func (w Widget) evaluate(d Data, now time.Time) (Shown, []bool) {
	s := Shown{Mode: w.Mode, Title: w.Title, Label: w.Label, Unit: w.Unit}
	if w.Decimals >= 0 {
		decimals := w.Decimals
		s.Decimals = &decimals
	}
	matched := make([]bool, len(w.Rules))
	switch {
	case d.Missing:
		s.State = StateMissing
		return s, matched
	case !d.HasValue && d.Failing:
		s.State = StateFailing
		return s, matched
	case !d.HasValue:
		s.State = StateLoading
		return s, matched
	}
	s.State, s.Stale, s.Failing = StateOK, d.Stale, d.Failing
	env := Env{Doc: d.Doc, Now: now}
	switch w.Mode {
	case ModeValue:
		v, _ := w.Path.Resolve(d.Doc)
		s.Value = cell(v)
		env.Vars = map[string]any{"value": v}
	case ModeKeyValues:
		for _, p := range w.Pairs {
			v, _ := p.Path.Resolve(d.Doc)
			s.Pairs = append(s.Pairs, ShownPair{Label: p.Label, Value: cell(v)})
		}
	case ModeBadges:
		for _, p := range w.Pairs {
			if v, _ := p.Path.Resolve(d.Doc); isOn(v) {
				s.Badges = append(s.Badges, p.Label)
			}
		}
	case ModeList:
		s.Rows = w.rows(d.Doc, now, matched)
	}
	for i, r := range w.Rules {
		if r.perRow(w.Mode) {
			continue
		}
		ok, _ := r.When.Matches(env)
		matched[i] = ok
		if ok {
			s.Tone = stronger(s.Tone, r.Tone)
			s.Hidden = s.Hidden || r.Hide
		}
	}
	// The first matching rule with a message decides it, so the order of
	// the rules is their priority.
	for i, r := range w.Rules {
		if matched[i] && r.Message != "" {
			s.Message = r.Message
			break
		}
	}
	return s, matched
}

// rows reads the list and applies the rules that read row to each item.
// Every item is checked, so a rule holds when any item matches, but only
// the first rows that are not hidden are sent.
func (w Widget) rows(doc any, now time.Time, matched []bool) []ShownRow {
	v, _ := w.Path.Resolve(doc)
	items, _ := v.([]any)
	rows := []ShownRow{}
	for _, item := range items {
		row := ShownRow{Cells: make([]any, len(w.Fields))}
		hidden := false
		env := Env{Doc: doc, Vars: map[string]any{"row": item}, Now: now}
		for i, r := range w.Rules {
			if !r.perRow(w.Mode) {
				continue
			}
			if ok, _ := r.When.Matches(env); ok {
				matched[i] = true
				row.Tone = stronger(row.Tone, r.Tone)
				hidden = hidden || r.Hide
			}
		}
		if hidden || len(rows) == maxRows {
			continue
		}
		for i, field := range w.Fields {
			value, _ := field.Resolve(item)
			row.Cells[i] = cell(value)
		}
		rows = append(rows, row)
	}
	return rows
}

// onWords are the texts that turn a badge on, as APIs such as Home
// Assistant send a state: a person is "home", a switch is "on".
var onWords = []string{"true", "on", "yes", "home", "open"}

// isOn reports whether a badge with value v shows: true, a number other
// than 0, or one of onWords in any case. Anything else, and a value that
// is missing, leaves it off.
func isOn(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return slices.Contains(onWords, strings.ToLower(strings.TrimSpace(x)))
	}
	return false
}

var toneWeight = map[string]int{"": 0, ToneOK: 1, ToneWarn: 2, ToneBad: 3}

// stronger returns the more serious tone, so the order of rules does not
// matter for colours: a bad value never shows as merely a warning.
func stronger(a, b string) string {
	if toneWeight[b] > toneWeight[a] {
		return b
	}
	return a
}

// cell turns a value from the answer into something a TV shows as it is:
// numbers, true, false and null stay, text is cut short, and a list or
// object becomes short text.
func cell(v any) any {
	switch x := v.(type) {
	case nil, float64, bool:
		return x
	case string:
		return clip(x)
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			text := ""
			switch c := cell(item).(type) {
			case string:
				text = c
			default:
				b, _ := json.Marshal(c)
				text = string(b)
			}
			parts = append(parts, text)
			if len(parts) > maxCellRunes/2 {
				break
			}
		}
		return clip(strings.Join(parts, ", "))
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return clip(string(b))
}

// clip makes text one line of at most maxCellRunes characters.
func clip(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	if utf8.RuneCountInString(s) <= maxCellRunes {
		return s
	}
	return string([]rune(s)[:maxCellRunes-1]) + "…"
}
