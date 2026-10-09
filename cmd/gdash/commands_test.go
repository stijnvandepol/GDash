package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthURL(t *testing.T) {
	tests := []struct{ addr, want string }{
		{":8420", "http://127.0.0.1:8420/healthz"},
		{"0.0.0.0:8420", "http://127.0.0.1:8420/healthz"},
		{"[::]:9000", "http://127.0.0.1:9000/healthz"},
		{"192.168.1.5:8420", "http://192.168.1.5:8420/healthz"},
		{"[fe80::1]:8420", "http://[fe80::1]:8420/healthz"},
		{"localhost:8420", "http://localhost:8420/healthz"},
	}
	for _, tt := range tests {
		got, err := healthURL(tt.addr)
		if err != nil || got != tt.want {
			t.Errorf("healthURL(%q) = %q, %v; want %q", tt.addr, got, err, tt.want)
		}
	}
	if _, err := healthURL("8420"); err == nil {
		t.Error("healthURL without a port: want an error")
	}
}

func TestHealthcheck(t *testing.T) {
	status := http.StatusNoContent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path = %q, want /healthz", r.URL.Path)
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	getenv := env(map[string]string{"GDASH_ADDR": strings.TrimPrefix(server.URL, "http://")})

	if err := healthcheck(context.Background(), getenv); err != nil {
		t.Fatalf("healthcheck of a healthy server: %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := healthcheck(context.Background(), getenv); err == nil {
		t.Error("healthcheck of an unhealthy server: want an error")
	}
	server.Close()
	if err := healthcheck(context.Background(), getenv); err == nil {
		t.Error("healthcheck without a server: want an error")
	}
}
