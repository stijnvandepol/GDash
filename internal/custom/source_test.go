package custom

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
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

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "gdash.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC) }
	return &Service{Store: db, Box: box, Now: now, Logger: slog.New(slog.DiscardHandler)}
}

func TestServiceSealsSecretsAndNeverShowsThem(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	view, err := svc.Add(ctx, input(func(in *Input) {
		in.Auth.Kind, in.Auth.Name, in.Auth.Secret = AuthQuery, "appid", testSecret
		in.Headers = []Header{{Name: "X-Plain", Value: "shown"}, {Name: "X-Hidden", Value: testSecret + "-h", Secret: true}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	shown, _ := json.Marshal(view)
	list, _ := svc.List(ctx)
	listed, _ := json.Marshal(list)
	row, _ := svc.Store.CustomSource(ctx, view.ID)
	for what, text := range map[string]string{"view": string(shown), "list": string(listed), "row": row.Headers + row.URL + row.SecretsSealed} {
		if strings.Contains(text, testSecret) {
			t.Errorf("the %s holds the secret: %s", what, text)
		}
	}
	if !view.Auth.SecretSet || view.Headers[0].Value != "shown" || view.Headers[1].Value != "" || !view.Headers[1].Secret {
		t.Errorf("view = %+v", view)
	}

	src, err := svc.Source(ctx, view.ID)
	if err != nil || src.AuthSecret != testSecret || src.Headers[1].Value != testSecret+"-h" || src.Interval != 5*time.Minute {
		t.Fatalf("Source = %+v, %v", src, err)
	}

	// An edit without the secrets keeps them.
	if _, err := svc.Update(ctx, view.ID, input(func(in *Input) {
		in.Name = "Renamed"
		in.Auth.Kind, in.Auth.Name = AuthQuery, "appid"
		in.Headers = []Header{{Name: "X-Hidden", Secret: true}}
	})); err != nil {
		t.Fatal(err)
	}
	all, err := svc.Sources(ctx)
	if err != nil || len(all) != 1 || all[0].Name != "Renamed" || all[0].AuthSecret != testSecret || all[0].Headers[0].Value != testSecret+"-h" {
		t.Fatalf("Sources after update = %+v, %v", all, err)
	}
	if err := svc.Delete(ctx, view.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, view.ID, input(nil)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update of a deleted source: %v", err)
	}
}

func TestServiceWithAnotherKey(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	view, err := svc.Add(ctx, input(func(in *Input) { in.Auth.Kind, in.Auth.Secret = AuthBearer, "tok" }))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := secret.NewBox([]byte(strings.Repeat("k", 32)))
	svc.Box = other
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 || !list[0].Unreadable || list[0].Auth.SecretSet {
		t.Fatalf("List with another key = %+v, %v", list, err)
	}
	if all, err := svc.Sources(ctx); err != nil || len(all) != 0 {
		t.Fatalf("Sources with another key = %+v, %v; want the source left out", all, err)
	}
	// Entering the token again repairs it.
	if _, err := svc.Update(ctx, view.ID, input(func(in *Input) { in.Auth.Kind, in.Auth.Secret = AuthBearer, "new" })); err != nil {
		t.Fatal(err)
	}
	if all, _ := svc.Sources(ctx); len(all) != 1 || all[0].AuthSecret != "new" {
		t.Fatalf("Sources after a new token = %+v", all)
	}
}

func TestServiceLimit(t *testing.T) {
	ctx := context.Background()
	svc := testService(t)
	for range MaxSources {
		if _, err := svc.Add(ctx, input(nil)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Add(ctx, input(nil)); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("over the limit: %v", err)
	}
}
