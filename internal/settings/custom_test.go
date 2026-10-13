package settings

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
)

func TestCustomSourceNamedValues(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	view, err := svc.AddCustomSource(ctx, customInput(nil))
	if err != nil {
		t.Fatal(err)
	}
	if view.Values == nil || view.Lists == nil || len(view.Values) != 0 {
		t.Fatalf("a new source has values %#v and lists %#v, want empty lists", view.Values, view.Lists)
	}
	in := custom.Values{
		Values: []custom.NamedValue{{Name: "Outside", Path: "current.temp", Unit: "°C"}},
		Lists:  []custom.NamedList{{Name: "Days", Path: "daily", Columns: []custom.Column{{Name: "Day", Path: "date"}}}},
	}
	named, err := svc.SetCustomSourceValues(ctx, view.ID, in)
	if err != nil || len(named.Values) != 1 || named.Values[0].Unit != "°C" || named.Lists[0].Columns[0].Path != "date" {
		t.Fatalf("SetCustomSourceValues = %+v, %v", named, err)
	}

	// Editing the source keeps them.
	edited, err := svc.UpdateCustomSource(ctx, view.ID, customInput(func(in *custom.Input) { in.Name = "Renamed" }))
	if err != nil || len(edited.Values) != 1 || len(edited.Lists) != 1 {
		t.Fatalf("after an edit = %+v, %v", edited, err)
	}
	list, _ := svc.CustomSources(ctx)
	if len(list[0].Values) != 1 || list[0].Values[0].Name != "Outside" {
		t.Fatalf("list = %+v", list)
	}

	in.Values = append(in.Values, custom.NamedValue{Name: "outside", Path: "x"})
	if _, err := svc.SetCustomSourceValues(ctx, view.ID, in); len(validation.Fields(err)) != 1 {
		t.Fatalf("a name used twice: %v", err)
	}
	if _, err := svc.SetCustomSourceValues(ctx, view.ID+1, custom.Values{}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("values of an unknown source: %v", err)
	}
}

const apiKey = "s3cr3t-key-123"

func customInput(mutate func(*custom.Input)) custom.Input {
	in := custom.Input{Name: "Weather", URL: "https://api.example.com/v1?city=oss", Method: "GET", IntervalSeconds: 300}
	in.Auth.Kind = custom.AuthNone
	if mutate != nil {
		mutate(&in)
	}
	return in
}

func TestCustomSourceSecretsAreSealedAndNeverShown(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	view, err := svc.AddCustomSource(ctx, customInput(func(in *custom.Input) {
		in.Auth.Kind, in.Auth.Name, in.Auth.Secret = custom.AuthQuery, "appid", apiKey
		in.Headers = []custom.Header{{Name: "X-Plain", Value: "shown"}, {Name: "X-Hidden", Value: apiKey + "-h", Secret: true}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	shown, _ := json.Marshal(view)
	list, _ := svc.CustomSources(ctx)
	listed, _ := json.Marshal(list)
	row, _ := svc.Store.CustomSource(ctx, view.ID)
	for what, text := range map[string]string{"view": string(shown), "list": string(listed), "row": row.Headers + row.URL + row.SecretsSealed} {
		if strings.Contains(text, apiKey) {
			t.Errorf("the %s holds the secret: %s", what, text)
		}
	}
	if !view.Auth.SecretSet || view.Headers[0].Value != "shown" || view.Headers[1].Value != "" || !view.Headers[1].Secret {
		t.Errorf("view = %+v", view)
	}

	src, err := svc.CustomSourceWithSecrets(ctx, view.ID)
	if err != nil || src.AuthSecret != apiKey || src.Headers[1].Value != apiKey+"-h" || src.Interval != 5*time.Minute {
		t.Fatalf("Source = %+v, %v", src, err)
	}

	// An edit without the secrets keeps them.
	if _, err := svc.UpdateCustomSource(ctx, view.ID, customInput(func(in *custom.Input) {
		in.Name = "Renamed"
		in.Auth.Kind, in.Auth.Name = custom.AuthQuery, "appid"
		in.Headers = []custom.Header{{Name: "X-Hidden", Secret: true}}
	})); err != nil {
		t.Fatal(err)
	}
	all, err := svc.FetchableCustomSources(ctx)
	if err != nil || len(all) != 1 || all[0].Name != "Renamed" || all[0].AuthSecret != apiKey || all[0].Headers[0].Value != apiKey+"-h" {
		t.Fatalf("Sources after update = %+v, %v", all, err)
	}
	if err := svc.DeleteCustomSource(ctx, view.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateCustomSource(ctx, view.ID, customInput(nil)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update of a deleted source: %v", err)
	}
}

func TestCustomSourceLoginIsShownWithoutThePassword(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	login := custom.Login{URL: "http://pi.hole/api/auth", Body: `{"password": "{secret}"}`, TokenPath: "session.sid", Place: custom.TokenInHeader, ValidityPath: "session.validity"}
	withLogin := func(password string) func(*custom.Input) {
		return func(in *custom.Input) {
			in.URL = "http://pi.hole/api/stats/summary"
			in.Auth.Kind, in.Auth.Name, in.Auth.Secret, in.Auth.Login = custom.AuthLogin, "X-FTL-SID", password, &login
		}
	}
	view, err := svc.AddCustomSource(ctx, customInput(withLogin(apiKey)))
	if err != nil {
		t.Fatal(err)
	}
	shown, _ := json.Marshal(view)
	row, _ := svc.Store.CustomSource(ctx, view.ID)
	if strings.Contains(string(shown), apiKey) || strings.Contains(row.Login+row.SecretsSealed, apiKey) {
		t.Fatalf("the view or the row holds the password: %s %+v", shown, row)
	}
	if view.Auth.Login == nil || *view.Auth.Login != login || !view.Auth.SecretSet {
		t.Fatalf("view auth = %+v, want the login and that the password is set", view.Auth)
	}
	// An edit without the password keeps it.
	if _, err := svc.UpdateCustomSource(ctx, view.ID, customInput(withLogin(""))); err != nil {
		t.Fatal(err)
	}
	src, err := svc.CustomSourceWithSecrets(ctx, view.ID)
	if err != nil || src.AuthSecret != apiKey || src.Login != login {
		t.Fatalf("Source = %+v, %v", src, err)
	}
	// Another kind of authentication drops the login.
	if _, err := svc.UpdateCustomSource(ctx, view.ID, customInput(nil)); err != nil {
		t.Fatal(err)
	}
	if row, _ := svc.Store.CustomSource(ctx, view.ID); row.Login != "" {
		t.Fatalf("login after switching to none = %q", row.Login)
	}
}

func TestCustomSourceWithAnotherKey(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	view, err := svc.AddCustomSource(ctx, customInput(func(in *custom.Input) { in.Auth.Kind, in.Auth.Secret = custom.AuthBearer, "tok" }))
	if err != nil {
		t.Fatal(err)
	}
	other, _ := secret.NewBox([]byte(strings.Repeat("k", 32)))
	svc.Box = other
	list, err := svc.CustomSources(ctx)
	if err != nil || len(list) != 1 || !list[0].Unreadable || list[0].Auth.SecretSet {
		t.Fatalf("List with another key = %+v, %v", list, err)
	}
	if all, err := svc.FetchableCustomSources(ctx); err != nil || len(all) != 0 {
		t.Fatalf("Sources with another key = %+v, %v; want the source left out", all, err)
	}
	// Entering the token again repairs it.
	if _, err := svc.UpdateCustomSource(ctx, view.ID, customInput(func(in *custom.Input) { in.Auth.Kind, in.Auth.Secret = custom.AuthBearer, "new" })); err != nil {
		t.Fatal(err)
	}
	if all, _ := svc.FetchableCustomSources(ctx); len(all) != 1 || all[0].AuthSecret != "new" {
		t.Fatalf("Sources after a new token = %+v", all)
	}
}

func TestCustomSourcePresetIsListed(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	own, err := svc.AddCustomSource(ctx, customInput(nil))
	if err != nil || own.Preset != "" {
		t.Fatalf("own source = %+v, %v", own, err)
	}
	made, err := svc.AddCustomSource(ctx, customInput(func(in *custom.Input) { in.Name, in.Preset = "P1 meter", "homewizard-p1" }))
	if err != nil || made.Preset != "homewizard-p1" {
		t.Fatalf("source from a preset = %+v, %v", made, err)
	}
	list, err := svc.CustomSources(ctx)
	if err != nil || len(list) != 2 || list[0].Preset != "" || list[1].Preset != "homewizard-p1" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	// The web interface tells its own APIs apart by an empty preset, so the
	// field is always sent.
	shown, _ := json.Marshal(list[0])
	if !strings.Contains(string(shown), `"preset":""`) {
		t.Errorf("an own source is listed as %s, without an empty preset", shown)
	}
	if fetched, _ := svc.CustomSourceWithSecrets(ctx, made.ID); fetched.Preset != "homewizard-p1" {
		t.Errorf("source with secrets has preset %q", fetched.Preset)
	}
	if _, err := svc.AddCustomSource(ctx, customInput(func(in *custom.Input) { in.Preset = "Not A Preset" })); len(validation.Fields(err)) != 1 {
		t.Fatalf("an invalid preset: %v", err)
	}
}

func TestTrialCustomSourceKeepsStoredSecretsWithoutStoring(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	view, err := svc.AddCustomSource(ctx, customInput(func(in *custom.Input) { in.Auth.Kind, in.Auth.Secret = custom.AuthBearer, apiKey }))
	if err != nil {
		t.Fatal(err)
	}
	changed := customInput(func(in *custom.Input) {
		in.URL = "https://api.example.com/v2"
		in.Auth.Kind = custom.AuthBearer
	})
	trial, err := svc.TrialCustomSource(ctx, view.ID, changed)
	if err != nil || trial.ID != 0 || trial.AuthSecret != apiKey || trial.URL != "https://api.example.com/v2" {
		t.Fatalf("trial of a change = %+v, %v; want the new address with the stored token and no ID", trial, err)
	}
	if stored, _ := svc.CustomSourceWithSecrets(ctx, view.ID); stored.URL != "https://api.example.com/v1?city=oss" {
		t.Fatalf("a trial changed the stored source: %+v", stored)
	}
	// A new source has no stored token to keep.
	if _, err := svc.TrialCustomSource(ctx, 0, changed); len(validation.Fields(err)) != 1 {
		t.Fatalf("a new source without its token: %v", err)
	}
	if _, err := svc.TrialCustomSource(ctx, view.ID+1, changed); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a trial of an unknown source: %v", err)
	}
	if list, _ := svc.CustomSources(ctx); len(list) != 1 {
		t.Fatalf("a trial stored a source: %+v", list)
	}
}

func TestCustomSourceLimit(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)
	for range custom.MaxSources {
		if _, err := svc.AddCustomSource(ctx, customInput(nil)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.AddCustomSource(ctx, customInput(nil)); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("over the limit: %v", err)
	}
}
