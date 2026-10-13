package custom

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/outbound"
)

const testSecret = "s3cr3t-key-123"

func testFetcher() *Fetcher {
	return &Fetcher{HTTP: outbound.NewClient(5 * time.Second), UserAgent: "GDash/test"}
}

// mustNotLeak fails when an error, as logged or shown, holds the secret.
func mustNotLeak(t *testing.T, err error) {
	t.Helper()
	var fetchErr *FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("err = %v (%T), want a *FetchError", err, err)
	}
	if strings.Contains(err.Error(), testSecret) || strings.Contains(fetchErr.Message, testSecret) {
		t.Fatalf("the error holds the secret: %v", err)
	}
}

func TestFetchSendsEachKindOfAuthentication(t *testing.T) {
	cases := []struct {
		src   Source
		check func(r *http.Request) bool
	}{
		{Source{Auth: AuthNone}, func(r *http.Request) bool { return r.Header.Get("Authorization") == "" }},
		{Source{Auth: AuthBearer, AuthSecret: testSecret}, func(r *http.Request) bool {
			return r.Header.Get("Authorization") == "Bearer "+testSecret
		}},
		{Source{Auth: AuthHeader, AuthName: "X-API-Key", AuthSecret: testSecret}, func(r *http.Request) bool {
			return r.Header.Get("X-API-Key") == testSecret
		}},
		{Source{Auth: AuthQuery, AuthName: "appid", AuthSecret: testSecret}, func(r *http.Request) bool {
			return r.URL.Query().Get("appid") == testSecret && r.URL.Query().Get("q") == "oss"
		}},
		{Source{Auth: AuthBasic, AuthName: "gdash", AuthSecret: testSecret}, func(r *http.Request) bool {
			user, pass, ok := r.BasicAuth()
			return ok && user == "gdash" && pass == testSecret
		}},
	}
	for _, c := range cases {
		var ok bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok = c.check(r) && r.Header.Get("User-Agent") == "GDash/test" && r.Header.Get("X-Extra") == "yes" &&
				r.Header.Get("Accept") == "application/json"
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = io.WriteString(w, `{ "temp" : 21.5 }`)
		}))
		c.src.URL, c.src.Method = srv.URL+"/data?q=oss", "GET"
		c.src.Headers = []Header{{Name: "X-Extra", Value: "yes"}}
		got, err := testFetcher().Fetch(context.Background(), c.src)
		srv.Close()
		if err != nil || string(got) != `{"temp":21.5}` || !ok {
			t.Errorf("%s: Fetch = %s, %v; request as expected: %v", c.src.Auth, got, err, ok)
		}
	}
}

func TestFetchPostsTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(body) != `{"query":"x"}` || r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `[1,2]`)
	}))
	defer srv.Close()
	got, err := testFetcher().Fetch(context.Background(), Source{URL: srv.URL, Method: "POST", Auth: AuthNone, Body: `{"query":"x"}`})
	if err != nil || string(got) != `[1,2]` {
		t.Fatalf("Fetch = %s, %v", got, err)
	}
}

func TestFetchFailuresNeverShowTheSecret(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"status 401": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "bad key "+testSecret, http.StatusUnauthorized)
		},
		"text/html": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html>"+testSecret)
		},
		"not valid JSON": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"key": "`+testSecret+`"`)
		},
		"larger than 1 MB": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `"`+strings.Repeat("x", MaxResponseBytes)+`"`)
		},
		"another host": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.invalid/steal?"+r.URL.RawQuery, http.StatusFound)
		},
	}
	for want, handler := range handlers {
		srv := httptest.NewServer(handler)
		_, err := testFetcher().Fetch(context.Background(), Source{
			URL: srv.URL, Method: "GET", Auth: AuthQuery, AuthName: "key", AuthSecret: testSecret,
		})
		srv.Close()
		mustNotLeak(t, err)
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to mention %q", err, want)
		}
	}
}

func TestFetchFollowsARedirectOnTheSameHost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/new", http.StatusMovedPermanently) })
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != testSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"moved":true}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	got, err := testFetcher().Fetch(context.Background(), Source{
		URL: srv.URL + "/old", Method: "GET", Auth: AuthHeader, AuthName: "X-API-Key", AuthSecret: testSecret,
	})
	if err != nil || string(got) != `{"moved":true}` {
		t.Fatalf("Fetch = %s, %v", got, err)
	}
}

func TestFetchStopsEndlessRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
	}))
	defer srv.Close()
	_, err := testFetcher().Fetch(context.Background(), Source{URL: srv.URL + "/a", Method: "GET", Auth: AuthNone})
	mustNotLeak(t, err)
}

func TestFetchTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := testFetcher().Fetch(ctx, Source{URL: srv.URL + "/?key=" + testSecret, Method: "GET", Auth: AuthBearer, AuthSecret: testSecret})
	mustNotLeak(t, err)
	if !strings.Contains(err.Error(), "did not answer in time") {
		t.Errorf("err = %v, want a time-out", err)
	}
}

func TestFetchRefusesMetadataAddresses(t *testing.T) {
	_, err := testFetcher().Fetch(context.Background(), Source{
		URL: "http://169.254.169.254/latest/meta-data/", Method: "GET", Auth: AuthBearer, AuthSecret: testSecret,
	})
	mustNotLeak(t, err)
	if !strings.Contains(err.Error(), "metadata") {
		t.Errorf("err = %v, want the address refused", err)
	}
}

func TestFetchAcceptsJSONWithoutAJSONContentType(t *testing.T) {
	for _, contentType := range []string{"", "text/plain", "application/geo+json", "text/json"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header()["Content-Type"] = []string{contentType}
			_, _ = io.WriteString(w, `{"a":1}`)
		}))
		_, err := testFetcher().Fetch(context.Background(), Source{URL: srv.URL, Method: "GET", Auth: AuthNone})
		srv.Close()
		if err != nil {
			t.Errorf("Content-Type %q: %v", contentType, err)
		}
	}
}

func TestReadAnswer(t *testing.T) {
	got, err := ReadAnswer(strings.NewReader("{ \"a\" : [1, 2] }\n"))
	if err != nil || string(got) != `{"a":[1,2]}` {
		t.Fatalf("ReadAnswer = %s, %v; want compact JSON", got, err)
	}
	limit := `"` + strings.Repeat("x", MaxResponseBytes-2) + `"`
	if _, err := ReadAnswer(strings.NewReader(limit)); err != nil {
		t.Fatalf("an answer of exactly 1 MB: %v", err)
	}
	if _, err := ReadAnswer(strings.NewReader(limit + " ")); !errors.Is(err, ErrAnswerTooLarge) {
		t.Fatalf("an answer over 1 MB: %v, want ErrAnswerTooLarge", err)
	}
	for _, bad := range []string{"", "{", "1 2", "<html>"} {
		if _, err := ReadAnswer(strings.NewReader(bad)); !errors.Is(err, ErrAnswerNotJSON) {
			t.Errorf("ReadAnswer(%q) = %v, want ErrAnswerNotJSON", bad, err)
		}
	}
}
