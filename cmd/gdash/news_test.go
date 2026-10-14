package main

import (
	"slices"
	"testing"

	"github.com/stijnvandepol/gdash/internal/news"
)

func TestNewsFeedsComeFromTheStandInOnlyInTests(t *testing.T) {
	nos := news.Catalogue()[0]
	feeds := []news.Source{{ID: 1, Name: nos.Name, URL: nos.URL}, {ID: 2, Name: "Mine", URL: "http://127.0.0.1:9/own.xml"}}

	if got := (&dataSources{}).newsFeeds(feeds); !slices.Equal(got, feeds) {
		t.Fatalf("without a stand-in: %+v; want the feeds as they are", got)
	}

	d := &dataSources{newsStandIn: "http://127.0.0.1:18700" + testNewsPath}
	got := d.newsFeeds(feeds)
	if got[0].URL != "http://127.0.0.1:18700/news/"+nos.ID || got[0].ID != 1 || got[0].Name != nos.Name {
		t.Fatalf("catalogue feed = %+v; want it from the stand-in under its catalogue id", got[0])
	}
	if got[1] != feeds[1] {
		t.Fatalf("own feed = %+v; want it unchanged", got[1])
	}
	if feeds[0].URL != nos.URL {
		t.Fatal("the settings' list was changed")
	}
}
