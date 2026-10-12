package custom

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var exprNow = time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

func evalString(t *testing.T, src string, doc any, vars map[string]any) (any, error) {
	t.Helper()
	e, err := ParseExpr(src)
	if err != nil {
		t.Fatalf("ParseExpr(%q): %v", src, err)
	}
	return e.Eval(Env{Doc: doc, Vars: vars, Now: exprNow})
}

func TestExpressionValues(t *testing.T) {
	doc := decode(t, sampleAnswer)
	cases := map[string]any{
		"1 + 2 * 3":                      7.0,
		"(1 + 2) * 3":                    9.0,
		"10 / 4 - 1":                     1.5,
		"-2 * -3":                        6.0,
		"1e3 + 0.5":                      1000.5,
		"7 - 2 - 1":                      4.0,
		"8 / 2 / 2":                      2.0,
		`"a" + 'b'`:                      "ab",
		`"say \"hi\""`:                   `say "hi"`,
		"true and false or true":         true,
		"not true":                       false,
		"!false && true":                 true,
		"1 < 2 and 2 <= 2 and 3 > 2":     true,
		"2 >= 3":                         false,
		`"apple" < "banana"`:             true,
		"null == null":                   true,
		"data.items[0].name == \"Milk\"": true,
		"data.items[2].price == null":    true,
		"data.missing == null":           true,
		"data.items[1].price * 2":        5.0,
		"len(data.items)":                3.0,
		"len(\"héllo\")":                 5.0,
		"len(data)":                      1.0,
		"contains(data.items[*].name, \"Bread\")": true,
		"contains(\"GDash\", \"Da\")":             true,
		"contains(data.items[*].price, 9)":        false,
		"lower(\"ABC\") + upper(\"d\")":           "abcD",
		"round(2.567, 2)":                         2.57,
		"round(2.5)":                              3.0,
		"number(\"21.5\") + 1":                    22.5,
		"number(4)":                               4.0,
		"isOn(\"Home\") and isOn(1) and isOn(ok)": true,
		"isOn(0) or isOn(\"away\") or isOn(null)": false,
		"minutesSince(\"2026-10-12T11:30:00Z\")":  30.0,
		"minutesSince(\"2026-10-12T12:10:00\")":   -10.0,
		"minutesSince(\"2026-10-12\")":            720.0,
		"minutesSince(1791800000)":                exprNow.Sub(time.Unix(1791800000, 0)).Minutes(),
		"minutesSince(1791800000000)":             exprNow.Sub(time.Unix(1791800000, 0)).Minutes(),
		"data.items == data.items":                false,
		"data.items != null":                      true,
		"\"3\" == 3":                              false,
		"ok":                                      true,
		"$[\"2m\"] > 20":                          true,
	}
	for src, want := range cases {
		got, err := evalString(t, src, doc, nil)
		if err != nil || got != want {
			t.Errorf("%s = %#v, %v; want %#v", src, got, err, want)
		}
	}
}

func TestExpressionVariables(t *testing.T) {
	got, err := evalString(t, "value > 20 and row.name == \"x\"", nil,
		map[string]any{"value": 21.0, "row": map[string]any{"name": "x"}})
	if err != nil || got != true {
		t.Fatalf("got %v, %v", got, err)
	}
	e, _ := ParseExpr("row.price > 2 or value == 1 or $.row == 1")
	if !e.Reads("row") || !e.Reads("value") || e.Reads("price") {
		t.Errorf("Reads: row %v, value %v, price %v", e.Reads("row"), e.Reads("value"), e.Reads("price"))
	}
}

func TestShortCircuitSkipsTheRightSide(t *testing.T) {
	doc := decode(t, `{}`)
	for _, src := range []string{"x != null and x > 3", "x == null or x > 3"} {
		got, err := evalString(t, src, doc, nil)
		want := strings.Contains(src, "or")
		if err != nil || got != want {
			t.Errorf("%s = %v, %v; want %v without an error", src, got, err, want)
		}
	}
}

func TestEvaluationErrors(t *testing.T) {
	doc := decode(t, sampleAnswer)
	for _, src := range []string{
		"data.missing > 3", "1 + \"a\"", "\"a\" + 1", "-\"a\"", "1 / 0", "not 1", "1 and true",
		"data.items < 3", "1e308 * 10", "len(1)", "lower(1)", "round(\"a\")", "round(1, 7)", "round(1, 1.5)",
		"number(\"abc\")", "number(true)", "minutesSince(\"yesterday\")", "minutesSince(true)", "contains(1, 1)",
		"contains(\"a\", 1)",
	} {
		e, err := ParseExpr(src)
		if err != nil {
			t.Errorf("ParseExpr(%q): %v", src, err)
			continue
		}
		if v, err := e.Eval(Env{Doc: doc, Now: exprNow}); err == nil {
			t.Errorf("%s = %v, want an error", src, v)
		}
	}
}

func TestMatchesNeedsTrueOrFalse(t *testing.T) {
	e, _ := ParseExpr("value")
	if _, err := e.Matches(Env{Vars: map[string]any{"value": 3.0}}); err == nil {
		t.Error("a rule that yields a number matched without an error")
	}
	e, _ = ParseExpr("value > 2")
	if ok, err := e.Matches(Env{Vars: map[string]any{"value": 3.0}}); !ok || err != nil {
		t.Errorf("value > 2 with 3 = %v, %v", ok, err)
	}
}

func TestJoinedTextIsBounded(t *testing.T) {
	long := strings.Repeat("x", maxTextResult)
	if _, err := evalString(t, "v + v", nil, map[string]any{"v": long}); err == nil {
		t.Error("joining two long texts was allowed")
	}
}

func TestSyntaxErrors(t *testing.T) {
	cases := map[string]int{
		"":                 1,
		"1 +":              4,
		"(1 + 2":           7,
		"1 2":              3,
		"a == b == c":      8,
		"foo(1)":           1,
		"len()":            1,
		"len(1, 2)":        1,
		"round(1, 2, 3)":   1,
		"len(1 2)":         7,
		"\"open":           1,
		"1 # 2":            3,
		"a.":               1,
		"and":              1,
		"1 and or 2":       7,
		"x[":               1,
		"= 1":              1,
		"$.x(1)":           4,
		"1 <> 2":           4,
		"'bad \\q escape'": 1,
	}
	for src, pos := range cases {
		_, err := ParseExpr(src)
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("ParseExpr(%q) = %v, want a syntax error", src, err)
			continue
		}
		if syntax.Position != pos {
			t.Errorf("ParseExpr(%q) error %q at %d, want at %d", src, syntax.Message, syntax.Position, pos)
		}
	}
}

func TestExpressionLimits(t *testing.T) {
	if _, err := ParseExpr(strings.Repeat("1+", MaxExpressionLength/2) + "1"); err == nil {
		t.Error("an expression over the length limit was parsed")
	}
	deep := strings.Repeat("(", maxNesting+1) + "1" + strings.Repeat(")", maxNesting+1)
	if _, err := ParseExpr(deep); err == nil {
		t.Error("an expression nested too deeply was parsed")
	}
	if _, err := ParseExpr(strings.Repeat("not ", maxNesting+1) + "true"); err == nil {
		t.Error("a chain of not deeper than the limit was parsed")
	}
	if _, err := ParseExpr(strings.Repeat("(", maxNesting) + "1" + strings.Repeat(")", maxNesting)); err != nil {
		t.Errorf("nesting at the limit: %v", err)
	}
	// A long flat chain is not nesting: it stays within the length limit.
	if _, err := ParseExpr(strings.Repeat("1 + ", 100) + "1"); err != nil {
		t.Errorf("a long sum: %v", err)
	}
}
