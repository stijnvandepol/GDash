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
	Rule   *struct {
		For  string `json:"for"`
		When string `json:"when"`
	} `json:"rule"`
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

func TestPresetRulesWorkOnTheirSample(t *testing.T) {
	for id, p := range loadPresets(t) {
		if p.Rule == nil {
			continue
		}
		expr, err := ParseExpr(p.Rule.When)
		if err != nil {
			t.Errorf("%s: the rule does not parse: %v", id, err)
			continue
		}
		var doc any
		if err := json.Unmarshal(p.Source.Sample, &doc); err != nil {
			t.Fatal(err)
		}
		vars := map[string]any{}
		for _, v := range p.Values {
			if v.Name == p.Rule.For {
				vars["value"], _ = mustPath(t, v.Path).Resolve(doc)
			}
		}
		for _, l := range p.Lists {
			if l.Name == p.Rule.For {
				items, _ := mustPath(t, l.Path).Resolve(doc)
				vars["row"] = items.([]any)[0]
			}
		}
		if len(vars) == 0 {
			t.Errorf("%s: the rule is for %q, which is no named value or list", id, p.Rule.For)
			continue
		}
		if _, err := expr.Matches(Env{Doc: doc, Vars: vars, Now: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}); err != nil {
			t.Errorf("%s: the rule fails on the sample: %v", id, err)
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
