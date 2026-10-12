package custom

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func decode(t testing.TB, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

const sampleAnswer = `{
	"data": {"items": [
		{"name": "Milk", "price": 1.25, "tags": ["dairy"]},
		{"name": "Bread", "price": 2.5, "tags": []},
		{"name": "Eggs", "price": null}
	]},
	"weird key": {"a.b": 7},
	"2m": 21,
	"ok": true
}`

func TestPathResolve(t *testing.T) {
	doc := decode(t, sampleAnswer)
	cases := []struct {
		path  string
		want  any
		found bool
	}{
		{"data.items[0].name", "Milk", true},
		{"data.items[-1].name", "Eggs", true},
		{"data.items[1].price", 2.5, true},
		{"data.items[2].price", nil, true},
		{"data.items[3].name", nil, false},
		{"data.items[-4]", nil, false},
		{"data.items[*].name", []any{"Milk", "Bread", "Eggs"}, true},
		{"data.items[*].tags[*]", []any{"dairy"}, true},
		{"data.items[*].missing", []any{}, true},
		{`["weird key"]["a.b"]`, 7.0, true},
		{`$['2m']`, 21.0, true},
		{"$.ok", true, true},
		{"ok", true, true},
		{"nothing", nil, false},
		{"data.items.name", nil, false},
		{"ok[0]", nil, false},
	}
	for _, c := range cases {
		p, err := ParsePath(c.path)
		if err != nil {
			t.Errorf("ParsePath(%q): %v", c.path, err)
			continue
		}
		got, found := p.Resolve(doc)
		if found != c.found || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %#v, %v; want %#v, %v", c.path, got, found, c.want, c.found)
		}
		if p.String() != c.path {
			t.Errorf("String() = %q, want %q", p.String(), c.path)
		}
	}
}

func TestPathOnATopLevelList(t *testing.T) {
	doc := decode(t, `[{"id": 1}, {"id": 2}]`)
	for path, want := range map[string]any{"[1].id": 2.0, "$[0].id": 1.0, "$": doc} {
		p, err := ParsePath(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := p.Resolve(doc); !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", path, got, want)
		}
	}
}

func TestPathRejectsBadSyntax(t *testing.T) {
	for _, bad := range []string{
		"", "data.", ".data", "data..x", "data[", "data[x]", "data[1", "data[*", `data["x]`, `data["x"`,
		"2m", "data-x", "data items", "$x", "data[1.5]", "data[99999999999]", `data["\q"]`,
		strings.Repeat("a", MaxPathLength+1), "a" + strings.Repeat("[0]", maxSegments),
	} {
		if _, err := ParsePath(bad); err == nil {
			t.Errorf("ParsePath(%q) accepted a bad path", bad)
		}
	}
}

func TestPathStepsAreBounded(t *testing.T) {
	if _, err := ParsePath("a" + strings.Repeat("[0]", maxSegments-1)); err != nil {
		t.Fatalf("a path of %d steps: %v", maxSegments, err)
	}
}

func TestPathWithVariables(t *testing.T) {
	p, err := ParsePath("row.price")
	if err != nil {
		t.Fatal(err)
	}
	doc := decode(t, `{"row": {"price": 1}}`)
	if got, _ := p.resolveFrom(doc, map[string]any{"row": map[string]any{"price": 9.0}}); got != 9.0 {
		t.Errorf("row.price with a row = %v, want the row's 9", got)
	}
	if got, _ := p.resolveFrom(doc, nil); got != 1.0 {
		t.Errorf("row.price without a row = %v, want the answer's 1", got)
	}
	rooted, _ := ParsePath("$.row.price")
	if got, _ := rooted.resolveFrom(doc, map[string]any{"row": map[string]any{"price": 9.0}}); got != 1.0 {
		t.Errorf("$.row.price = %v; $ must reach the answer past a variable", got)
	}
}
