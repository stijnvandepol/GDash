package custom

import (
	"strings"
	"testing"
	"time"
)

// FuzzExpr feeds the parser and evaluator anything. Rules are typed by an
// admin and stored with dashboards, so neither may panic, hang or take
// long, whatever the text. Run longer with:
//
//	go test ./internal/custom -run '^$' -fuzz FuzzExpr -fuzztime 60s
func FuzzExpr(f *testing.F) {
	for _, seed := range []string{
		"value > 25 and value < 30",
		"row.price >= 2.5 or not contains(lower(row.name), \"milk\")",
		`data.items[*].name`, `$["weird key"]['a.b'] == 7`, "minutesSince(updated) > 15",
		"round(number(\"21.456\"), 2) == 21.46", "len(data.items) * 2 / 3 - -1",
		"((((1))))", "!(!true) && false || null == x[-1]", "1e308 * 1e308", "\"a\" + \"b\" + 'c'",
		strings.Repeat("(", 30) + "1" + strings.Repeat(")", 30),
		strings.Repeat("-", 40) + "1", "x[*][*][*].y", "len(", "'\\", "$", "[0]", "a.b.c.d.e.f",
	} {
		f.Add(seed)
	}
	doc := decode(f, sampleAnswer)
	vars := map[string]any{"value": 21.5, "row": map[string]any{"name": "Milk", "price": 1.25}}
	f.Fuzz(func(t *testing.T, src string) {
		start := time.Now()
		e, err := ParseExpr(src)
		if err != nil {
			if len(src) <= MaxExpressionLength && !strings.Contains(err.Error(), "character") {
				t.Fatalf("ParseExpr(%q) error %q does not say where", src, err)
			}
			return
		}
		if len(src) > MaxExpressionLength {
			t.Fatalf("ParseExpr accepted %d characters", len(src))
		}
		_, _ = e.Eval(Env{Doc: doc, Vars: vars, Now: exprNow})
		_, _ = e.Matches(Env{Doc: nil, Now: exprNow})
		if took := time.Since(start); took > time.Second {
			t.Fatalf("ParseExpr and Eval of %q took %v", src, took)
		}
	})
}

// FuzzPath does the same for the path syntax.
func FuzzPath(f *testing.F) {
	for _, seed := range []string{"data.items[0].name", "$", "[*][-1]", `["a\"b"]`, "a.", "a[", "x" + strings.Repeat("[*]", 31)} {
		f.Add(seed)
	}
	doc := decode(f, sampleAnswer)
	f.Fuzz(func(t *testing.T, src string) {
		p, err := ParsePath(src)
		if err != nil {
			return
		}
		if p.String() != src {
			t.Fatalf("ParsePath(%q).String() = %q", src, p.String())
		}
		_, _ = p.Resolve(doc)
	})
}
