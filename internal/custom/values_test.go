package custom

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/validation"
)

func TestCheckValuesAcceptsNamedValuesAndLists(t *testing.T) {
	one := 1
	in := Values{
		Values: []NamedValue{{Name: "Outside", Path: "current.temperature_2m", Unit: "°C", Decimals: &one}, {Name: "Sky", Path: `weather[0]["main text"]`}},
		Lists:  []NamedList{{Name: "Releases", Path: "$", Columns: []Column{{Name: "Version", Path: "tag_name"}, {Name: "Item", Path: "$"}}}},
	}
	got, err := CheckValues(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Values) != 2 || got.Values[0].Unit != "°C" || *got.Values[0].Decimals != 1 || len(got.Lists[0].Columns) != 2 {
		t.Fatalf("CheckValues = %+v", got)
	}
	empty, err := CheckValues(Values{})
	if err != nil || empty.Values == nil || empty.Lists == nil {
		t.Fatalf("CheckValues of nothing = %#v, %v; want empty lists", empty, err)
	}
}

func TestCheckValuesNamesEveryProblem(t *testing.T) {
	seven := 7
	in := Values{
		Values: []NamedValue{
			{Name: "", Path: "a"},
			{Name: "Temp", Path: "a..b"},
			{Name: "temp", Path: "b", Unit: "a unit far too long", Decimals: &seven},
			{Name: strings.Repeat("x", 41), Path: "c"},
			{Name: "Line\nbreak", Path: "d"},
		},
		Lists: []NamedList{
			{Name: "TEMP", Path: "items"},
			{Name: "Items", Path: "items", Columns: []Column{{Name: "A", Path: "x"}, {Name: "a", Path: "[*"}}},
		},
	}
	_, err := CheckValues(in)
	var fields []string
	for _, f := range validation.Fields(err) {
		fields = append(fields, f.Field)
	}
	want := []string{
		"values[0].name", "values[1].path", "values[2].name", "values[2].unit", "values[2].decimals", "values[3].name",
		"values[4].name", "lists[0].name", "lists[0].columns", "lists[1].columns[1].name", "lists[1].columns[1].path",
	}
	if !slices.Equal(fields, want) {
		t.Fatalf("fields = %v\nwant     %v", fields, want)
	}
}

func TestCheckValuesLimits(t *testing.T) {
	var in Values
	for i := range MaxNamedValues + 1 {
		in.Values = append(in.Values, NamedValue{Name: fmt.Sprint("v", i), Path: "a"})
	}
	for i := range MaxNamedLists + 1 {
		in.Lists = append(in.Lists, NamedList{Name: fmt.Sprint("l", i), Path: "a", Columns: []Column{{Name: "c", Path: "c"}}})
	}
	in.Lists[0].Columns = []Column{{Name: "a", Path: "a"}, {Name: "b", Path: "b"}, {Name: "c", Path: "c"}, {Name: "d", Path: "d"}}
	_, err := CheckValues(in)
	fields := validation.Fields(err)
	if len(fields) != 2 || fields[0].Field != "values" || fields[1].Field != "lists" {
		t.Fatalf("fields = %+v", fields)
	}
	in.Values, in.Lists = in.Values[:1], in.Lists[:1]
	if fields := validation.Fields(func() error { _, err := CheckValues(in); return err }()); len(fields) != 1 || fields[0].Field != "lists[0].columns" {
		t.Fatalf("four columns: fields = %+v", fields)
	}
}
