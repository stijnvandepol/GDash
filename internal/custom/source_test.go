package custom

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
)

func input(mutate func(*Input)) Input {
	in := Input{Name: "Weather", URL: "https://api.example.com/v1?city=oss", Method: "GET", IntervalSeconds: 300}
	in.Auth.Kind = AuthNone
	if mutate != nil {
		mutate(&in)
	}
	return in
}

func fieldsOf(err error) []string {
	var names []string
	for _, f := range validation.Fields(err) {
		names = append(names, f.Field)
	}
	return names
}

func TestCheckAcceptsEachKindOfSource(t *testing.T) {
	for _, mutate := range []func(*Input){
		nil,
		func(in *Input) { in.Auth.Kind, in.Auth.Secret = AuthBearer, "tok.en-1" },
		func(in *Input) {
			in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthHeader, "X-API-Key", "key with space"
		},
		func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthHeader, "Authorization", "Token abc" },
		func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthQuery, "appid", "abc" },
		func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthBasic, "me", "pass word" },
		func(in *Input) { in.Method, in.Body = "post", `{"a": [1]}` },
		func(in *Input) {
			in.Headers = []Header{{Name: "Accept-Language", Value: "nl"}, {Name: "X-Key", Value: "k", Secret: true}}
		},
		func(in *Input) { in.Sample = json.RawMessage(` {"a": 1} `) },
		func(in *Input) { in.URL = "http://192.168.1.10:8123/api" },
		func(in *Input) { in.IntervalSeconds = 30 },
		func(in *Input) { in.IntervalSeconds = 86400 },
	} {
		if _, err := Check(input(mutate), nil); err != nil {
			t.Errorf("Check(%+v): %v", input(mutate), err)
		}
	}
}

func TestCheckNamesEveryInvalidField(t *testing.T) {
	cases := []struct {
		field  string
		mutate func(*Input)
	}{
		{"name", func(in *Input) { in.Name = " x" }},
		{"url", func(in *Input) { in.URL = "ftp://example.com" }},
		{"method", func(in *Input) { in.Method = "DELETE" }},
		{"body", func(in *Input) { in.Body = "{}" }},
		{"intervalSeconds", func(in *Input) { in.IntervalSeconds = 29 }},
		{"auth.kind", func(in *Input) { in.Auth.Kind = "cookie" }},
		{"auth.secret", func(in *Input) { in.Auth.Kind = AuthBearer }},
		{"auth.name", func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthHeader, "Bad Name", "k" }},
		{"headers[0].name", func(in *Input) { in.Headers = []Header{{Name: "Host", Value: "evil"}} }},
		{"headers[1].name", func(in *Input) { in.Headers = []Header{{Name: "X-A", Value: "1"}, {Name: "x-a", Value: "2"}} }},
		{"headers[0].value", func(in *Input) { in.Headers = []Header{{Name: "X-A", Value: "a\r\nX-B: b"}} }},
		{"headers", func(in *Input) { in.Headers = make([]Header, maxHeaders+1) }},
		{"sample", func(in *Input) { in.Sample = json.RawMessage(`{"a":`) }},
		{"auth.name", func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthQuery, "city", "k" }},
		{"auth.secret", func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthBasic, "me", "a\nb" }},
		{"url", func(in *Input) { in.URL = "https://me:pw@example.com" }},
		{"body", func(in *Input) { in.Method, in.Body = "POST", "not json" }},
		{"intervalSeconds", func(in *Input) { in.IntervalSeconds = 86401 }},
		{"auth.name", func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthBasic, "a:b", "pw" }},
		{"headers[0].name", func(in *Input) { in.Headers = []Header{{Name: "Authorization", Value: "x"}} }},
		{"auth.secret", func(in *Input) { in.Auth.Kind, in.Auth.Secret = AuthBearer, "has space" }},
		{"url", func(in *Input) { in.URL = "https://example.com/#top" }},
		{"auth.name", func(in *Input) { in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthHeader, "Content-Length", "1" }},
	}
	for _, c := range cases {
		_, err := Check(input(c.mutate), nil)
		if got := fieldsOf(err); !slices.Contains(got, c.field) {
			t.Errorf("%s: fields %v", c.field, got)
		}
	}
}

func TestCheckKeepsStoredSecrets(t *testing.T) {
	old := &Source{
		ID: 4, Auth: AuthHeader, AuthName: "X-API-Key", AuthSecret: "old-key",
		Headers: []Header{{Name: "X-Tenant", Value: "old-tenant", Secret: true}},
	}
	src, err := Check(input(func(in *Input) {
		in.Auth.Kind, in.Auth.Name = AuthHeader, "X-API-Key"
		in.Headers = []Header{{Name: "x-tenant", Secret: true}}
	}), old)
	if err != nil || src.AuthSecret != "old-key" || src.Headers[0].Value != "old-tenant" || src.ID != 4 {
		t.Fatalf("Check = %+v, %v; want the stored secrets kept", src, err)
	}
	// Another kind of authentication needs its own secret.
	_, err = Check(input(func(in *Input) { in.Auth.Kind, in.Auth.Name = AuthQuery, "key" }), old)
	if !slices.Contains(fieldsOf(err), "auth.secret") {
		t.Fatalf("a header key became a query key without asking: %v", err)
	}
}

func pushInput(mutate func(*Input)) Input {
	in := Input{Name: "Homey", Mode: ModePush, IntervalSeconds: 60}
	if mutate != nil {
		mutate(&in)
	}
	return in
}

func TestCheckPushedSource(t *testing.T) {
	old := &Source{ID: 3, Mode: ModeFetch, Auth: AuthBearer, AuthSecret: "old-token", URL: "https://x"}
	src, err := Check(pushInput(func(in *Input) { in.Auth.Kind, in.Method = AuthNone, "GET" }), old)
	want := Source{ID: 3, Name: "Homey", Mode: ModePush, Method: "GET", Auth: AuthNone, Interval: time.Minute}
	if err != nil || src.Mode != want.Mode || src.URL != "" || src.Auth != AuthNone || src.AuthSecret != "" || src.Method != "GET" ||
		src.ID != 3 || src.Interval != time.Minute || len(src.Headers) != 0 {
		t.Fatalf("Check = %+v, %v; want %+v without the old token", src, err, want)
	}
	if src, err := Check(input(nil), nil); err != nil || src.Mode != ModeFetch {
		t.Fatalf("a source without a mode = %+v, %v; want one that fetches", src, err)
	}

	cases := []struct {
		field  string
		mutate func(*Input)
	}{
		{"url", func(in *Input) { in.URL = "https://example.com" }},
		{"method", func(in *Input) { in.Method = "POST" }},
		{"body", func(in *Input) { in.Body = "{}" }},
		{"auth.kind", func(in *Input) { in.Auth.Kind, in.Auth.Secret = AuthBearer, "t" }},
		{"auth.kind", func(in *Input) { in.Auth.Secret = "t" }},
		{"headers", func(in *Input) { in.Headers = []Header{{Name: "X-A", Value: "1"}} }},
		{"intervalSeconds", func(in *Input) { in.IntervalSeconds = 10 }},
		{"mode", func(in *Input) { in.Mode = "poll" }},
	}
	for _, c := range cases {
		_, err := Check(pushInput(c.mutate), nil)
		if got := fieldsOf(err); !slices.Contains(got, c.field) {
			t.Errorf("%s: fields %v", c.field, got)
		}
	}
}

func TestPushFreshFor(t *testing.T) {
	if got := PushFreshFor(time.Minute); got != 150*time.Second {
		t.Fatalf("PushFreshFor(1m) = %v, want 2m30s", got)
	}
}
