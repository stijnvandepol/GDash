package news

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func parseFile(t *testing.T, name string, maxItems int) Feed {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	feed, err := Parse(f, maxItems)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return feed
}

func TestParseRSS(t *testing.T) {
	feed := parseFile(t, "rss.xml", 10)
	if feed.Title != "Example News" {
		t.Fatalf("title = %q", feed.Title)
	}
	want := []Item{
		{Title: "Trains run again after storm & floods", Link: "https://example.org/1", Published: time.Date(2026, 10, 8, 17, 17, 58, 0, time.UTC)},
		{Title: "Second headline across lines", Link: "", Published: time.Date(2026, 10, 8, 18, 6, 27, 0, time.UTC)},
		{Title: "Headline without a date", Link: "https://example.org/3"},
	}
	if len(feed.Items) != len(want) {
		t.Fatalf("items = %+v, want %d (the empty title is skipped)", feed.Items, len(want))
	}
	for i := range want {
		if feed.Items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, feed.Items[i], want[i])
		}
	}
}

func TestParseRSSLimitsItems(t *testing.T) {
	if got := parseFile(t, "rss.xml", 1).Items; len(got) != 1 {
		t.Fatalf("items = %d, want 1", len(got))
	}
}

func TestParseAtom(t *testing.T) {
	feed := parseFile(t, "atom.xml", 10)
	want := []Item{
		{Title: "Release 2.0 is out", Link: "https://example.org/release", Published: time.Date(2026, 10, 8, 15, 30, 0, 0, time.UTC)},
		{Title: "Published wins over updated", Link: "https://example.org/two", Published: time.Date(2026, 10, 7, 7, 0, 0, 0, time.UTC)},
	}
	if feed.Title != "Example Atom" || len(feed.Items) != 2 || feed.Items[0] != want[0] || feed.Items[1] != want[1] {
		t.Fatalf("feed = %+v, want items %+v", feed, want)
	}
}

func TestParseRejectsOtherDocuments(t *testing.T) {
	for _, doc := range []string{"<html><body>not a feed</body></html>", "plain text", ""} {
		if _, err := Parse(strings.NewReader(doc), 5); err == nil {
			t.Errorf("Parse(%q) accepted a non-feed", doc)
		}
	}
}

func TestPlainTextCutsLongTitles(t *testing.T) {
	got := plainText(strings.Repeat("é", 300))
	if n := len([]rune(got)); n != maxTitleRunes {
		t.Fatalf("length = %d runes, want %d", n, maxTitleRunes)
	}
}

func TestFetchUsesConditionalRequests(t *testing.T) {
	body, err := os.ReadFile("testdata/rss.xml")
	if err != nil {
		t.Fatal(err)
	}
	var requests, notModified atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Daycast") {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Write(body)
	}))
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Timeout: 5 * time.Second}}

	first, err := c.Fetch(context.Background(), srv.URL, 5)
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	second, err := c.Fetch(context.Background(), srv.URL, 5)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if notModified.Load() != 1 || len(second.Items) != len(first.Items) {
		t.Fatalf("304s = %d, items %d vs %d; want the cached feed after a 304", notModified.Load(), len(second.Items), len(first.Items))
	}
}

func TestFetchRejectsErrorsAndHugeFeeds(t *testing.T) {
	huge := "<rss><channel>" + strings.Repeat("<item><title>x</title></item>", maxFeedBytes/20) + "</channel></rss>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(huge))
	}))
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Timeout: 5 * time.Second}}

	if _, err := c.Fetch(context.Background(), srv.URL+"/missing", 5); err == nil {
		t.Fatal("Fetch accepted a 404")
	}
	if _, err := c.Fetch(context.Background(), srv.URL+"/huge", 5); !errors.Is(err, errFeedTooLarge) {
		t.Fatalf("Fetch of a huge feed: err = %v, want errFeedTooLarge", err)
	}
}

func TestHeadlinesMergesFeedsNewestFirst(t *testing.T) {
	feeds := map[string]string{
		"/a": `<rss><channel><title>A</title>
			<item><title>A old</title><pubDate>Thu, 08 Oct 2026 08:00:00 +0000</pubDate></item>
			<item><title>A undated</title></item></channel></rss>`,
		"/b": `<rss><channel><title>B</title>
			<item><title>B new</title><pubDate>Thu, 08 Oct 2026 09:00:00 +0000</pubDate></item></channel></rss>`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := feeds[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()
	c := &Client{HTTP: &http.Client{Timeout: 5 * time.Second}}
	ctx := context.Background()

	single, err := c.Headlines(ctx, []Source{{Name: "Mine", URL: srv.URL + "/a"}}, 5)
	if err != nil || single.Title != "A" || single.Items[0].Source != "" {
		t.Fatalf("one feed = %+v, %v; want its own title and no source labels", single, err)
	}

	merged, err := c.Headlines(ctx, []Source{{Name: "Feed A", URL: srv.URL + "/a"}, {Name: "Feed B", URL: srv.URL + "/b"}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Title != "" || len(merged.Items) != 2 {
		t.Fatalf("merged = %+v", merged)
	}
	if merged.Items[0].Title != "B new" || merged.Items[0].Source != "Feed B" || merged.Items[1].Title != "A old" {
		t.Fatalf("order = %q (%s), %q; want newest first with its feed name", merged.Items[0].Title, merged.Items[0].Source, merged.Items[1].Title)
	}

	if _, err := c.Headlines(ctx, []Source{{Name: "A", URL: srv.URL + "/a"}, {Name: "Gone", URL: srv.URL + "/gone"}}, 5); err == nil {
		t.Fatal("Headlines succeeded with a failing feed")
	}
	if _, cached := c.cached(srv.URL + "/b"); cached {
		t.Fatal("a removed feed is still cached")
	}
}
