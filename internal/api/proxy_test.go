package api

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestBehindProxies(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.17.0.1/32"), netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name       string
		remote     string
		forwarded  []string
		proto      string
		wantClient string
		wantHTTPS  bool
	}{
		{"direct client keeps its address", "192.168.1.50:5000", []string{"1.2.3.4"}, "https", "192.168.1.50", false},
		{"proxy passes the client on", "172.17.0.1:4000", []string{"192.168.1.50"}, "https", "192.168.1.50", true},
		{"client-written hops are skipped", "172.17.0.1:4000", []string{"6.6.6.6, 192.168.1.50, 10.0.0.2"}, "http", "192.168.1.50", false},
		{"several header lines count as one list", "172.17.0.1:4000", []string{"6.6.6.6", "192.168.1.50"}, "", "192.168.1.50", false},
		{"garbage keeps the proxy address", "172.17.0.1:4000", []string{"not an address"}, "HTTPS", "172.17.0.1", true},
		{"only proxies keeps the proxy address", "172.17.0.1:4000", []string{"10.0.0.2"}, "", "172.17.0.1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotClient string
			var gotHTTPS bool
			h := behindProxies(trusted, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotClient, gotHTTPS = clientIP(r), isHTTPS(r)
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for _, f := range tt.forwarded {
				req.Header.Add("X-Forwarded-For", f)
			}
			if tt.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.proto)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if gotClient != tt.wantClient || gotHTTPS != tt.wantHTTPS {
				t.Fatalf("client %s, https %v; want %s, %v", gotClient, gotHTTPS, tt.wantClient, tt.wantHTTPS)
			}
		})
	}
}

func TestSessionCookieIsSecureBehindAnHTTPSProxy(t *testing.T) {
	h := behindProxies([]netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "x", Secure: isHTTPS(r)})
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if cookies := rec.Result().Cookies(); len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("cookies = %v, want one Secure cookie", cookies)
	}
}
