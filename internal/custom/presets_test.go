package custom

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The presets for a custom API live with the web interface
// (web/src/admin/presets), which fills them in. This checks each against
// the server's own rules: a preset that this package would refuse, or whose
// named values miss its own sample answer, fails here instead of in a
// household's browser.

type preset struct {
	ID           string `json:"id"`
	Placeholders []struct {
		Key     string `json:"key"`
		Kind    string `json:"kind"`
		Example string `json:"example"`
	} `json:"placeholders"`
	Source struct {
		Name   string `json:"name"`
		URL    string `json:"url"`
		Method string `json:"method"`
		Body   string `json:"body"`
		Auth   struct {
			Kind  string `json:"kind"`
			Name  string `json:"name"`
			Login *Login `json:"login"`
		} `json:"auth"`
		Headers         []Header        `json:"headers"`
		IntervalSeconds int             `json:"intervalSeconds"`
		Sample          json.RawMessage `json:"sample"`
	} `json:"source"`
	Values []NamedValue `json:"values"`
	Lists  []NamedList  `json:"lists"`
	Views  []presetView `json:"views"`
}

// presetView is a ready-made Custom widget of a preset, which names the
// preset's values and list by name.
type presetView struct {
	ID     string `json:"id"`
	Mode   string `json:"mode"`
	Title  string `json:"title"`
	Values []struct {
		Name  string `json:"name"`
		Label string `json:"label"`
	} `json:"values"`
	List  string           `json:"list"`
	Rules []map[string]any `json:"rules"`
}

// settings are the widget settings the dashboard editor makes from the
// view (web/src/admin/presetWidget.ts), with numbers as JSON decodes them.
func (v presetView) settings(t *testing.T, p preset) map[string]any {
	t.Helper()
	named := func(name string) NamedValue {
		for _, value := range p.Values {
			if value.Name == name {
				return value
			}
		}
		t.Fatalf("%s, view %s: %q is no named value", p.ID, v.ID, name)
		return NamedValue{}
	}
	out := map[string]any{"source": float64(1), "mode": v.Mode}
	if v.Title != "" {
		out["title"] = v.Title
	}
	switch v.Mode {
	case ModeValue:
		if len(v.Values) != 1 {
			t.Fatalf("%s, view %s: value mode shows one value, not %d", p.ID, v.ID, len(v.Values))
		}
		value := named(v.Values[0].Name)
		out["path"] = value.Path
		if v.Values[0].Label != "" {
			out["label"] = v.Values[0].Label
		}
		if value.Unit != "" {
			out["unit"] = value.Unit
		}
		if value.Decimals != nil {
			out["decimals"] = float64(*value.Decimals)
		}
	case ModeKeyValues, ModeBadges:
		pairs := []any{}
		for _, shown := range v.Values {
			label := shown.Label
			if label == "" {
				label = shown.Name
			}
			pairs = append(pairs, map[string]any{"label": label, "path": named(shown.Name).Path})
		}
		out["pairs"] = pairs
	case ModeList:
		for _, l := range p.Lists {
			if l.Name == v.List {
				out["path"] = l.Path
				fields := []any{}
				for _, c := range l.Columns {
					fields = append(fields, c.Path)
				}
				out["fields"] = fields
			}
		}
		if out["path"] == nil {
			t.Fatalf("%s, view %s: %q is no named list", p.ID, v.ID, v.List)
		}
	}
	if len(v.Rules) > 0 {
		rules := []any{}
		for _, r := range v.Rules {
			rules = append(rules, r)
		}
		out["rules"] = rules
	}
	return out
}

func loadPresets(t *testing.T) map[string]preset {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "web", "src", "admin", "presets", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no presets found in web/src/admin/presets")
	}
	out := map[string]preset{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var p preset
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if p.ID+".json" != filepath.Base(file) {
			t.Errorf("%s: the id %q does not match the file name", file, p.ID)
		}
		out[p.ID] = p
	}
	return out
}

// filled is the preset's address, or another address of it such as its
// login's, with each placeholder's example, as the web interface fills it in.
func filled(p preset, address string) string {
	for _, ph := range p.Placeholders {
		value := ph.Example
		if ph.Kind == "id" || ph.Kind == "text" {
			value = url.QueryEscape(value)
		}
		address = strings.ReplaceAll(address, "{"+ph.Key+"}", value)
	}
	return address
}

func TestPresetsPassTheServersChecks(t *testing.T) {
	for id, p := range loadPresets(t) {
		in := Input{Name: p.Source.Name, Mode: ModeFetch, URL: filled(p, p.Source.URL), Method: p.Source.Method, Body: p.Source.Body,
			Headers: p.Source.Headers, IntervalSeconds: p.Source.IntervalSeconds, Sample: p.Source.Sample}
		in.Auth.Kind, in.Auth.Name = p.Source.Auth.Kind, p.Source.Auth.Name
		if login := p.Source.Auth.Login; login != nil {
			filledLogin := *login
			filledLogin.URL = filled(p, login.URL)
			in.Auth.Login = &filledLogin
		}
		if in.Auth.Kind != AuthNone {
			in.Auth.Secret = "preset-test-secret"
		}
		// The household types the user name of basic authentication.
		if in.Auth.Kind == AuthBasic && in.Auth.Name == "" {
			in.Auth.Name = "preset-test-user"
		}
		if strings.Contains(in.URL, "{") {
			t.Errorf("%s: a placeholder in %s has no entry under placeholders", id, p.Source.URL)
		}
		if _, err := Check(in, nil); err != nil {
			t.Errorf("%s: the source is refused: %v", id, err)
		}
		if _, err := CheckValues(Values{Values: p.Values, Lists: p.Lists}); err != nil {
			t.Errorf("%s: the named values are refused: %v", id, err)
		}
	}
}

func TestPresetValuesAreInTheirSample(t *testing.T) {
	for id, p := range loadPresets(t) {
		var doc any
		if err := json.Unmarshal(p.Source.Sample, &doc); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		for _, v := range p.Values {
			if _, found := mustPath(t, v.Path).Resolve(doc); !found {
				t.Errorf("%s: %s (%s) is not in the sample", id, v.Name, v.Path)
			}
		}
		for _, l := range p.Lists {
			items, _ := mustPath(t, l.Path).Resolve(doc)
			list, ok := items.([]any)
			if !ok || len(list) == 0 {
				t.Errorf("%s: %s (%s) is not a list with items in the sample", id, l.Name, l.Path)
				continue
			}
			for _, c := range l.Columns {
				if _, found := mustPath(t, c.Path).Resolve(list[0]); !found {
					t.Errorf("%s: column %s (%s) is not in the first item of %s", id, c.Name, c.Path, l.Name)
				}
			}
		}
	}
}

func TestPresetIDsAreAccepted(t *testing.T) {
	for id := range loadPresets(t) {
		if _, err := Check(input(func(in *Input) { in.Preset = id }), nil); err != nil {
			t.Errorf("%s: a source with this preset is refused: %v", id, err)
		}
	}
}

// Each view is a Custom widget that the server takes and that shows the
// preset's own sample, with rules that work on it.
func TestPresetViewsMakeWidgetsThatShowTheirSample(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	for id, p := range loadPresets(t) {
		if len(p.Views) == 0 {
			t.Errorf("%s: no views", id)
		}
		var doc any
		if err := json.Unmarshal(p.Source.Sample, &doc); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, v := range p.Views {
			if seen[v.ID] {
				t.Errorf("%s: view %s twice", id, v.ID)
			}
			seen[v.ID] = true
			w, err := ParseWidget(v.settings(t, p))
			if err != nil {
				t.Errorf("%s, view %s: the widget is refused: %v", id, v.ID, err)
				continue
			}
			shown := w.Show(Data{Doc: doc, HasValue: true}, now)
			if shown.State != StateOK || (v.Mode == ModeValue && shown.Value == nil) || (v.Mode == ModeList && len(shown.Rows) == 0 && len(w.Rules) == 0) {
				t.Errorf("%s, view %s: shows %+v for the sample", id, v.ID, shown)
			}
			for _, pair := range shown.Pairs {
				if pair.Value == nil {
					t.Errorf("%s, view %s: %s is not in the sample", id, v.ID, pair.Label)
				}
			}
			vars := map[string]any{}
			if v.Mode == ModeValue {
				vars["value"], _ = w.Path.Resolve(doc)
			}
			if v.Mode == ModeList {
				items, _ := w.Path.Resolve(doc)
				vars["row"] = items.([]any)[0]
			}
			for i, r := range w.Rules {
				if _, err := r.When.Matches(Env{Doc: doc, Vars: vars, Now: now}); err != nil {
					t.Errorf("%s, view %s: rule %d fails on the sample: %v", id, v.ID, i, err)
				}
			}
		}
	}
}

func mustPath(t *testing.T, text string) Path {
	t.Helper()
	p, err := ParsePath(text)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return p
}
