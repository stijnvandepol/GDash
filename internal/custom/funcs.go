package custom

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// function is one of the few functions a rule may call. They only read
// their arguments and the clock that the caller passes in.
type function struct {
	minArgs, maxArgs int
	call             func(args []any, now time.Time) (any, error)
}

func (f *function) arity() string {
	if f.minArgs == f.maxArgs {
		if f.minArgs == 1 {
			return "1 argument"
		}
		return fmt.Sprintf("%d arguments", f.minArgs)
	}
	return fmt.Sprintf("%d or %d arguments", f.minArgs, f.maxArgs)
}

// maxRoundDigits is as precise as a value on a TV needs to be.
const maxRoundDigits = 6

var functions = map[string]*function{
	"len":          {1, 1, length},
	"contains":     {2, 2, contains},
	"lower":        {1, 1, textFunc(strings.ToLower)},
	"upper":        {1, 1, textFunc(strings.ToUpper)},
	"round":        {1, 2, round},
	"number":       {1, 1, toNumber},
	"minutesSince": {1, 1, minutesSince},
}

func functionNames() string {
	names := make([]string, 0, len(functions))
	for name := range functions {
		names = append(names, name)
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// length counts the characters of a text or the items of a list or object.
func length(args []any, _ time.Time) (any, error) {
	switch v := args[0].(type) {
	case string:
		return float64(utf8.RuneCountInString(v)), nil
	case []any:
		return float64(len(v)), nil
	case map[string]any:
		return float64(len(v)), nil
	}
	return nil, fmt.Errorf("needs text, a list or an object, not %s", describe(args[0]))
}

// contains finds a text in a text, or a plain value in a list.
func contains(args []any, _ time.Time) (any, error) {
	switch v := args[0].(type) {
	case string:
		needle, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("looks for text in text, not for %s", describe(args[1]))
		}
		return strings.Contains(v, needle), nil
	case []any:
		for _, item := range v {
			if equal(item, args[1]) {
				return true, nil
			}
		}
		return false, nil
	}
	return nil, fmt.Errorf("looks in text or a list, not in %s", describe(args[0]))
}

func textFunc(f func(string) string) func([]any, time.Time) (any, error) {
	return func(args []any, _ time.Time) (any, error) {
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("needs text, not %s", describe(args[0]))
		}
		return f(s), nil
	}
}

// round rounds to whole numbers, or to the given number of decimals.
func round(args []any, _ time.Time) (any, error) {
	n, ok := args[0].(float64)
	if !ok {
		return nil, fmt.Errorf("needs a number, not %s", describe(args[0]))
	}
	digits := 0.0
	if len(args) == 2 {
		d, ok := args[1].(float64)
		if !ok || d != math.Trunc(d) || d < 0 || d > maxRoundDigits {
			return nil, fmt.Errorf("rounds to 0 to %d decimals", maxRoundDigits)
		}
		digits = d
	}
	scale := math.Pow(10, digits)
	return finite(math.Round(n*scale)/scale, nil)
}

// toNumber reads a number that an API sends as text, such as "21.5".
func toNumber(args []any, _ time.Time) (any, error) {
	switch v := args[0].(type) {
	case float64:
		return v, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, errors.New("the text is not a number")
		}
		return f, nil
	}
	return nil, fmt.Errorf("needs a number or text, not %s", describe(args[0]))
}

// unixMillisFrom tells milliseconds from seconds in a Unix time: in
// seconds, this is a date thousands of years from now.
const unixMillisFrom = 1e11

// minutesSince counts the minutes from a time to now; negative for a time
// still to come. It takes an ISO 8601 text, with or without a time and
// zone (UTC when absent), or a Unix time in seconds or milliseconds.
func minutesSince(args []any, now time.Time) (any, error) {
	var t time.Time
	switch v := args[0].(type) {
	case float64:
		if v >= unixMillisFrom {
			t = time.UnixMilli(int64(v))
		} else {
			t = time.Unix(int64(v), 0)
		}
	case string:
		parsed, err := parseTime(v)
		if err != nil {
			return nil, err
		}
		t = parsed
	default:
		return nil, fmt.Errorf("needs a time as text or a Unix time, not %s", describe(args[0]))
	}
	return now.Sub(t).Minutes(), nil
}

var timeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02"}

func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("the text is not a time such as 2026-10-12T08:30:00Z")
}
