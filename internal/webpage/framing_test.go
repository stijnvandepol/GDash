package webpage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func headers(pairs ...string) http.Header {
	h := http.Header{}
	for i := 0; i < len(pairs); i += 2 {
		h.Add(pairs[i], pairs[i+1])
	}
	return h
}

func TestEvaluateFrameOptions(t *testing.T) {
	for _, tc := range []struct {
		values  []string
		blocked bool
	}{
		{nil, false},
		{[]string{"DENY"}, true},
		{[]string{"sameorigin"}, true},
		{[]string{" SameOrigin "}, true},
		{[]string{"ALLOW-FROM http://192.168.1.2:8420"}, false},
		{[]string{"allowall"}, false},
		{[]string{"nonsense"}, false},
		{[]string{"deny", "deny"}, true},
		{[]string{"sameorigin, allowall"}, true},
		{[]string{"foo, bar"}, false},
	} {
		h := http.Header{}
		for _, v := range tc.values {
			h.Add("X-Frame-Options", v)
		}
		got := Evaluate(h, "http://grafana.local:3000", own)
		if (got.Blocked == BlockedByFrameOptions) != tc.blocked || (!tc.blocked && got.Blocked != "") {
			t.Errorf("%q: %+v, want blocked %v", tc.values, got, tc.blocked)
		}
	}
}

func TestEvaluateFrameAncestors(t *testing.T) {
	for policy, allowed := range map[string]bool{
		"frame-ancestors 'none'":                                   false,
		"frame-ancestors":                                          false,
		"frame-ancestors 'self'":                                   false,
		"default-src 'self'; frame-ancestors *":                    true,
		"frame-ancestors http:":                                    true,
		"frame-ancestors https:":                                   false,
		"frame-ancestors http://192.168.1.2:8420":                  true,
		"frame-ancestors 192.168.1.2:8420":                         true,
		"frame-ancestors 192.168.1.2:*":                            true,
		"frame-ancestors 192.168.1.2":                              false,
		"frame-ancestors http://192.168.1.2:8421":                  false,
		"frame-ancestors 'self' http://192.168.1.2:8420/d/":        true,
		"FRAME-ANCESTORS HTTP://192.168.1.2:8420":                  true,
		"script-src 'self'":                                        true,
		"frame-ancestors *, frame-ancestors 'self'":                false,
		"frame-ancestors https://192.168.1.2:8420 'unsafe-inline'": false,
	} {
		got := Evaluate(headers("Content-Security-Policy", policy), "http://grafana.local:3000", own)
		if (got.Blocked == "") != allowed {
			t.Errorf("%q: %+v, want allowed %v", policy, got, allowed)
		}
		if !allowed && got.Blocked != BlockedByFrameAncestors {
			t.Errorf("%q: blocked by %q", policy, got.Blocked)
		}
	}
}

func TestEvaluateFrameAncestorsWinsOverFrameOptions(t *testing.T) {
	h := headers("Content-Security-Policy", "frame-ancestors *", "X-Frame-Options", "DENY")
	if got := Evaluate(h, "https://ha.example.com", own); got.Blocked != "" {
		t.Errorf("got %+v, want allowed: frame-ancestors replaces X-Frame-Options", got)
	}
	// A report-only policy changes nothing.
	h = headers("Content-Security-Policy-Report-Only", "frame-ancestors *", "X-Frame-Options", "DENY")
	if got := Evaluate(h, "https://ha.example.com", own); got.Blocked != BlockedByFrameOptions {
		t.Errorf("got %+v, want blocked by X-Frame-Options", got)
	}
}

func TestEvaluateUpgradesHTTPSourcesForAnHTTPSGDash(t *testing.T) {
	h := headers("Content-Security-Policy", "frame-ancestors http://gdash.example.com")
	if got := Evaluate(h, "https://grafana.example.com", "https://gdash.example.com"); got.Blocked != "" {
		t.Errorf("got %+v, want allowed", got)
	}
	h = headers("Content-Security-Policy", "frame-ancestors gdash.example.com")
	if got := Evaluate(h, "http://grafana.local", "https://gdash.example.com"); got.Blocked != "" {
		t.Errorf("got %+v, want allowed: an http page's scheme upgrades to https", got)
	}
}

func TestEvaluateWildcardHosts(t *testing.T) {
	h := headers("Content-Security-Policy", "frame-ancestors https://*.example.com")
	if got := Evaluate(h, "https://grafana.example.com", "https://gdash.example.com"); got.Blocked != "" {
		t.Errorf("got %+v, want allowed", got)
	}
	if got := Evaluate(h, "https://grafana.example.com", "https://example.com"); got.Blocked == "" {
		t.Error("*.example.com matched example.com itself")
	}
}

func TestEvaluateShortensALongHeader(t *testing.T) {
	h := headers("Content-Security-Policy", "frame-ancestors 'none'; script-src "+strings.Repeat("x", 500))
	got := Evaluate(h, "http://a.local", own)
	if len(got.Header) > maxShownHeader+len("…") {
		t.Errorf("header of %d bytes", len(got.Header))
	}
}

func TestCheckFetchesThePageAndReadsItsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deny":
			w.Header().Set("X-Frame-Options", "DENY")
		case "/ancestors":
			w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
		case "/moved":
			http.Redirect(w, r, "/deny", http.StatusFound)
			return
		case "/missing":
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()
	checker := &Checker{HTTP: &http.Client{Timeout: 5 * time.Second}}
	for path, want := range map[string]Framing{
		"/":          {Status: 200},
		"/deny":      {Status: 200, Blocked: BlockedByFrameOptions, Header: "DENY"},
		"/ancestors": {Status: 200, Blocked: BlockedByFrameAncestors, Header: "frame-ancestors 'self'"},
		"/moved":     {Status: 200, Blocked: BlockedByFrameOptions, Header: "DENY"},
		"/missing":   {Status: 404},
	} {
		got, err := checker.Check(context.Background(), srv.URL+path+"?token=secret", own)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if got != want {
			t.Errorf("%s: %+v, want %+v", path, got, want)
		}
	}
}

func TestCheckNeverRepeatsTheQueryInItsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	address := srv.URL + "/?auth_token=secret"
	srv.Close()
	_, err := (&Checker{HTTP: &http.Client{Timeout: time.Second}}).Check(context.Background(), address, own)
	var checkErr *CheckError
	if !errors.As(err, &checkErr) {
		t.Fatalf("err = %v, want a CheckError", err)
	}
	if strings.Contains(err.Error(), "secret") || !strings.Contains(checkErr.Message, "could not be reached") {
		t.Errorf("error = %q", err)
	}
}
