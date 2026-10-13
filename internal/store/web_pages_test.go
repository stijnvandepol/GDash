package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/webpage"
)

func TestWebPageRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)
	page := webpage.Page{
		Name: "Grafana", URL: "http://192.168.1.10:3000/d/abc?kiosk", RefreshMinutes: 15, Zoom: 80, AllowScripts: true,
		CreatedAt: created, UpdatedAt: created,
	}
	id, err := s.AddWebPage(ctx, page, 2)
	if err != nil || id == 0 {
		t.Fatalf("AddWebPage = %d, %v", id, err)
	}
	page.ID = id
	if got, err := s.WebPage(ctx, id); err != nil || got != page {
		t.Fatalf("WebPage = %+v, %v; want %+v", got, err, page)
	}

	page.Name, page.AllowScripts, page.RefreshMinutes, page.UpdatedAt = "Radar", false, 0, created.Add(time.Hour)
	if err := s.UpdateWebPage(ctx, page); err != nil {
		t.Fatal(err)
	}
	list, err := s.WebPages(ctx)
	if err != nil || len(list) != 1 || list[0] != page {
		t.Fatalf("WebPages = %+v, %v; want [%+v]", list, err, page)
	}

	if _, err := s.AddWebPage(ctx, page, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddWebPage(ctx, page, 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("third page at a limit of 2: err = %v, want ErrConflict", err)
	}

	if err := s.DeleteWebPage(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WebPage(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted page: err = %v, want ErrNotFound", err)
	}
	if err := s.UpdateWebPage(ctx, page); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of a deleted page: err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteWebPage(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
}
