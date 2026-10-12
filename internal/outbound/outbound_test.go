package outbound

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"
)

func TestAllowed(t *testing.T) {
	for addr, want := range map[string]bool{
		"192.168.1.20":           true, // a NAS at home
		"10.0.0.5":               true,
		"127.0.0.1":              true, // Radicale on the same host
		"::1":                    true,
		"93.184.215.14":          true,
		"169.254.169.254":        false, // cloud instance metadata
		"::ffff:169.254.169.254": false,
		"fe80::1":                false,
		"0.0.0.0":                false,
		"::":                     false,
		"224.0.0.251":            false,
		"ff02::fb":               false,
		"fd00:ec2::254":          false, // AWS metadata over IPv6
		"fd12:3456::1":           true,  // another unique local address at home
		"64:ff9b::a9fe:a9fe":     false, // NAT64 form of 169.254.169.254
		"64:ff9b::5db8:d70e":     true,  // NAT64 form of a public address
		"2002:a9fe:a9fe::1":      false, // 6to4 form of 169.254.169.254
		"2002:5db8:d70e::1":      true,  // 6to4 form of a public address
		"2001:0:1:2::5601:5601":  false, // Teredo, client 169.254.169.254 inverted
		"::a9fe:a9fe":            false, // IPv4-compatible 169.254.169.254
		"100.100.100.200":        false, // Alibaba Cloud metadata
		"2606:4700::1111":        true,  // a public IPv6 address
	} {
		if got := Allowed(netip.MustParseAddr(addr)); got != want {
			t.Errorf("Allowed(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestClientConnectsToAllowedAndRefusesBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	client := NewClient(5 * time.Second)

	resp, err := client.Get(srv.URL) // loopback is allowed
	if err != nil {
		t.Fatalf("GET loopback: %v", err)
	}
	resp.Body.Close()

	if _, err := client.Get("http://169.254.169.254/latest/meta-data/"); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("GET metadata address: err = %v, want ErrBlockedAddress", err)
	}
}

func TestRedirectsMayNotLeaveHTTPS(t *testing.T) {
	https := func(scheme string) *http.Request {
		return &http.Request{URL: &url.URL{Scheme: scheme, Host: "calendar.example.org"}}
	}
	if err := checkRedirect(https("http"), []*http.Request{https("https")}); !errors.Is(err, ErrDowngrade) {
		t.Fatalf("https to http: err = %v, want ErrDowngrade", err)
	}
	if err := checkRedirect(https("https"), []*http.Request{https("http")}); err != nil {
		t.Fatalf("http to https: err = %v, want none", err)
	}
	if err := checkRedirect(https("http"), []*http.Request{https("http")}); err != nil {
		t.Fatalf("http to http: err = %v, want none", err)
	}
	many := make([]*http.Request, maxRedirects)
	for i := range many {
		many[i] = https("https")
	}
	if err := checkRedirect(https("https"), many); err == nil {
		t.Fatal("no error after the maximum number of redirects")
	}
}

func TestBaseURL(t *testing.T) {
	const hint = "paste the address, such as http://192.168.1.20"
	tests := []struct{ in, want, msg string }{
		{" http://192.168.1.10:8123/ ", "http://192.168.1.10:8123", ""},
		{"https://home.example.org/sub/", "https://home.example.org/sub", ""},
		{"192.168.1.20", "", hint},
		{"homey.local:80", "", hint},
		{"ftp://192.168.1.20", "", "use an http or https address"},
		{"http://user:pw@192.168.1.20", "", "use the address without a user name, ? or #"},
		{"http://192.168.1.20/?x=1", "", "use the address without a user name, ? or #"},
	}
	for _, tt := range tests {
		if got, msg := BaseURL(tt.in, hint); got != tt.want || msg != tt.msg {
			t.Errorf("BaseURL(%q) = %q, %q; want %q, %q", tt.in, got, msg, tt.want, tt.msg)
		}
	}
}
