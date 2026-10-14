package custom

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// Display modes of the Custom widget.
const (
	ModeValue     = "value"
	ModeList      = "list"
	ModeKeyValues = "keyValues"
	ModeBadges    = "badges"
)

// Modes lists the display modes, for the widget catalogue.
var Modes = []string{ModeValue, ModeList, ModeKeyValues, ModeBadges}

// pairMode reports whether mode shows a set of labelled paths.
func pairMode(mode string) bool { return mode == ModeKeyValues || mode == ModeBadges }

// Tones a rule can give a value or row; the TV shows them in the theme's
// own colours.
const (
	ToneOK   = "ok"
	ToneWarn = "warn"
	ToneBad  = "bad"
)

// Limits of one Custom widget, sized for a tile read from across a room.
const (
	MaxFields      = 3
	MaxPairs       = 8
	MaxRules       = 6
	MaxDecimals    = 6
	maxTitle       = 60
	maxLabel       = 40
	maxUnit        = 12
	maxRuleMessage = 120
)

// Widget is the parsed settings of a Custom widget.
type Widget struct {
	Source int64
	Mode   string
	Title  string
	// Path is the value in value mode and the list in list mode.
	Path  Path
	Label string
	Unit  string
	// Decimals is how many decimals a number shows, or -1 for as sent.
	Decimals int
	// Fields are read from each item of the list.
	Fields []Path
	Pairs  []Pair
	Rules  []Rule
}

// Pair is one line of key-values mode, or one badge.
type Pair struct {
	Label string
	Path  Path
}

// Rule changes how the widget shows its data while When holds.
type Rule struct {
	When     *Expr
	Tone     string
	Message  string
	Hide     bool
	Announce bool
}

// perRow reports whether the rule reads the row of a list, and so is
// evaluated for every row.
func (r Rule) perRow(mode string) bool { return mode == ModeList && r.When.Reads("row") }

// hideTitle is the setting every widget type takes (internal/display); the
// screen reads it, so it is only checked here.
var widgetKeys = []string{"source", "mode", "title", "hideTitle", "path", "label", "unit", "decimals", "fields", "pairs", "rules"}

// ParseWidget reads a Custom widget's settings as a dashboard stores them,
// with numbers as float64. It returns a *validation.Error whose field names
// the setting, such as rules[1].when.
func ParseWidget(settings map[string]any) (Widget, error) {
	var errs validation.Collector
	for _, key := range slices.Sorted(maps.Keys(settings)) {
		if !slices.Contains(widgetKeys, key) {
			errs.Add(key, "is not a setting of this widget")
		}
	}
	w := Widget{Decimals: -1}
	if id, ok := wholeNumber(settings["source"], 1, math.MaxInt32); ok {
		w.Source = int64(id)
	} else {
		errs.Add("source", "choose an API under Integrations, Custom APIs")
	}
	w.Mode, _ = settings["mode"].(string)
	if !slices.Contains(Modes, w.Mode) {
		errs.Add("mode", "choose value, list, keyValues or badges")
	}
	w.Title = optionalText(&errs, settings, "title", maxTitle)
	w.Label = optionalText(&errs, settings, "label", maxLabel)
	w.Unit = optionalText(&errs, settings, "unit", maxUnit)
	if v, set := settings["hideTitle"]; set {
		if _, ok := v.(bool); !ok {
			errs.Add("hideTitle", "use true or false")
		}
	}
	if v, set := settings["decimals"]; set {
		n, ok := wholeNumber(v, 0, MaxDecimals)
		if !ok {
			errs.Add("decimals", fmt.Sprintf("use a whole number from 0 to %d", MaxDecimals))
		}
		w.Decimals = n
	}
	if text, set := settings["path"]; set || !pairMode(w.Mode) {
		w.Path = parsePathSetting(&errs, "path", text)
	}
	w.Fields = parseFields(&errs, settings["fields"], w.Mode)
	w.Pairs = parsePairs(&errs, settings["pairs"], w.Mode)
	w.Rules = parseRules(&errs, settings["rules"])
	if err := errs.Err(); err != nil {
		return Widget{}, err
	}
	return w, nil
}

// wholeNumber reads a whole number from JSON within [lo, hi].
func wholeNumber(v any, lo, hi int) (int, bool) {
	f, ok := v.(float64)
	if !ok {
		if i, isInt := v.(int); isInt {
			f, ok = float64(i), true
		}
	}
	if !ok || f != math.Trunc(f) || f < float64(lo) || f > float64(hi) {
		return 0, false
	}
	return int(f), true
}

func optionalText(errs *validation.Collector, settings map[string]any, key string, limit int) string {
	v, set := settings[key]
	if !set {
		return ""
	}
	text, ok := v.(string)
	if !ok || !plainText(text, limit) {
		errs.Add(key, fmt.Sprintf("use at most %d characters on one line", limit))
	}
	return text
}

// plainText is text for one line on a TV: no control characters, no
// leading or trailing spaces, and at most limit characters.
func plainText(s string, limit int) bool {
	return utf8.RuneCountInString(s) <= limit && strings.TrimSpace(s) == s && !validation.HasControlCharacter(s)
}

func parsePathSetting(errs *validation.Collector, field string, v any) Path {
	text, ok := v.(string)
	if !ok || text == "" {
		errs.Add(field, "click a value in the data, or type a path such as data.items[0].name")
		return Path{}
	}
	p, err := ParsePath(text)
	if err != nil {
		errs.Add(field, err.Error())
	}
	return p
}

func parseFields(errs *validation.Collector, v any, mode string) []Path {
	list, ok := v.([]any)
	if v == nil && mode != ModeList {
		return nil
	}
	if !ok || len(list) > MaxFields || (mode == ModeList && len(list) == 0) {
		errs.Add("fields", fmt.Sprintf("choose 1 to %d values to show for each item", MaxFields))
		return nil
	}
	fields := make([]Path, len(list))
	for i, item := range list {
		fields[i] = parsePathSetting(errs, fmt.Sprintf("fields[%d]", i), item)
	}
	return fields
}

func parsePairs(errs *validation.Collector, v any, mode string) []Pair {
	list, ok := v.([]any)
	if v == nil && !pairMode(mode) {
		return nil
	}
	if !ok || len(list) > MaxPairs || (pairMode(mode) && len(list) == 0) {
		errs.Add("pairs", fmt.Sprintf("add 1 to %d lines with a label and a value", MaxPairs))
		return nil
	}
	pairs := make([]Pair, len(list))
	for i, item := range list {
		field := fmt.Sprintf("pairs[%d]", i)
		obj, _ := item.(map[string]any)
		label, _ := obj["label"].(string)
		if label == "" || !plainText(label, maxLabel) || len(obj) != 2 {
			errs.Add(field+".label", fmt.Sprintf("use 1 to %d characters on one line", maxLabel))
		}
		pairs[i] = Pair{Label: label, Path: parsePathSetting(errs, field+".path", obj["path"])}
	}
	return pairs
}

var tones = []string{"", ToneOK, ToneWarn, ToneBad}

func parseRules(errs *validation.Collector, v any) []Rule {
	if v == nil {
		return nil
	}
	list, ok := v.([]any)
	if !ok || len(list) > MaxRules {
		errs.Add("rules", fmt.Sprintf("use at most %d rules", MaxRules))
		return nil
	}
	rules := make([]Rule, 0, len(list))
	for i, item := range list {
		field := fmt.Sprintf("rules[%d]", i)
		obj, _ := item.(map[string]any)
		for key := range obj {
			if !slices.Contains([]string{"when", "tone", "message", "hide", "announce"}, key) {
				errs.Add(field+"."+key, "is not part of a rule")
			}
		}
		var r Rule
		when, _ := obj["when"].(string)
		expr, err := ParseExpr(when)
		if err != nil || when == "" {
			errs.Add(field+".when", "write a condition such as value > 25: "+errorText(err))
		}
		r.When = expr
		r.Tone, _ = obj["tone"].(string)
		if !slices.Contains(tones, r.Tone) {
			errs.Add(field+".tone", "choose ok, warn, bad or none")
		}
		r.Message, _ = obj["message"].(string)
		if !plainText(r.Message, maxRuleMessage) {
			errs.Add(field+".message", fmt.Sprintf("use at most %d characters on one line", maxRuleMessage))
		}
		r.Hide = flag(errs, obj, "hide", field)
		r.Announce = flag(errs, obj, "announce", field)
		if r.Announce && r.Message == "" {
			errs.Add(field+".message", "write the message to announce")
		}
		if r.Tone == "" && r.Message == "" && !r.Hide {
			errs.Add(field, "choose what the rule does: a tone, a message or hiding")
		}
		rules = append(rules, r)
	}
	return rules
}

// flag reads an optional true or false of a rule.
func flag(errs *validation.Collector, obj map[string]any, key, field string) bool {
	v, set := obj[key]
	b, ok := v.(bool)
	if set && !ok {
		errs.Add(field+"."+key, "use true or false")
	}
	return b
}

func errorText(err error) string {
	if err == nil {
		return "the condition is empty"
	}
	return err.Error()
}
