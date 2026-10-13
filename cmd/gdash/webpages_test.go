package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

func TestWebPagesFollowTheSettings(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if got := withWebPages(map[string]datahub.Entry{}, nil, logger); len(got) != 0 {
		t.Fatalf("data without web pages = %v, want no entry", got)
	}
	list := &webPageList{}
	list.set([]webpage.Page{{ID: 7, Name: "Grafana", URL: "http://192.168.1.10:3000/", Zoom: 100, AllowScripts: true}})
	got := withWebPages(map[string]datahub.Entry{}, list.get(), logger)
	var pages map[string]struct{ URL string }
	if err := json.Unmarshal(got[display.WebPageSource].Value, &pages); err != nil {
		t.Fatal(err)
	}
	if pages["7"].URL != "http://192.168.1.10:3000/" {
		t.Fatalf("web page entry = %s", got[display.WebPageSource].Value)
	}
}
