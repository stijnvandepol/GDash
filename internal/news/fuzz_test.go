package news

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzParse(f *testing.F) {
	for _, name := range []string{"rss.xml", "atom.xml"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data, uint8(10))
	}
	const rssHead = `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>`
	seeds := []string{
		"",
		"<",
		"<html><body>login</body></html>",
		`<?xml version="1.0" encoding="ISO-8859-1"?><rss><channel><title>caf` + "\xe9" + `</title></channel></rss>`,
		`<?xml version="1.0" encoding="UTF-16"?><rss/>`,
		`<?xml version="1.0" encoding="nonsense"?><rss/>`,
		"<rss>\x00<channel><title>nul\x00byte</title></channel></rss>",
		"<rss><channel><title>\xff\xfe invalid utf8 \xc3\x28</title></channel></rss>",
		rssHead + `<item><title><![CDATA[<b>bold</b> &amp; <script>alert(1)</script>]]></title><link>javascript:alert(1)</link></item></channel></rss>`,
		rssHead + `<item><title>&lt;img src=x onerror=alert(1)&gt; &amp;lt;</title><link>  https://example.org/a?b=c#d  </link></item></channel></rss>`,
		rssHead + `<item><title>t</title><link>data:text/html,x</link></item><item><title>u</title><link>//example.org/x</link></item><item><title>v</title><link>http:///x</link></item></channel></rss>`,
		rssHead + `<item><title>t</title><link>https://exa mple.org/\x00</link></item></channel></rss>`,
		rssHead + `<item><title>t</title><pubDate>Fri, 31 Dec 9999 23:00:00 -0100</pubDate></item></channel></rss>`,
		rssHead + `<item><title>t</title><pubDate>Mon, 1 Jan 0000 00:00:00 +0100</pubDate></item></channel></rss>`,
		rssHead + `<item><title>t</title><pubDate>Mon, 1 Jan 0001 00:00:00 +1400</pubDate></item></channel></rss>`,
		rssHead + `<item><title>t</title><pubDate>Thu, 8 Oct 2026 12:00:00 CEST</pubDate></item></channel></rss>`,
		rssHead + `<item><title>` + strings.Repeat("a", 100000) + `</title></item></channel></rss>`,
		rssHead + `<item><title>` + strings.Repeat("é", 100000) + `</title></item></channel></rss>`,
		rssHead + strings.Repeat(`<item><title>x</title></item>`, 20000) + `</channel></rss>`,
		`<?xml version="1.0"?><!DOCTYPE rss [<!ENTITY a "aaaaaaaaaa"><!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;">]><rss><channel><title>&b;&b;&b;</title></channel></rss>`,
		strings.Repeat("<a>", 100000),
		"<rss>" + strings.Repeat("<channel>", 50000) + strings.Repeat("</channel>", 50000) + "</rss>",
		`<feed xmlns="http://www.w3.org/2005/Atom"><title>A</title><entry><title>e</title><link rel="self" href="https://example.org/self"/><link href="https://example.org/alt"/><published>9999-12-31T23:59:59-01:00</published></entry></feed>`,
		`<feed><entry><title>e</title><updated>0000-01-01T00:00:00+23:59</updated><link rel="alternate" href="ftp://x"/></entry></feed>`,
		`<feed><entry><title>e</title><link href="HTTPS://EXAMPLE.ORG/%zz"/></entry></feed>`,
		`<feed><entry><title></title></entry><entry><title>   </title></entry></feed>`,
		`<rss><channel><item><title>a</title></item></channel></rss><rss><channel><item><title>b</title></item></channel></rss>`,
	}
	for _, s := range seeds {
		f.Add([]byte(s), uint8(3))
	}

	f.Fuzz(func(t *testing.T, data []byte, limit uint8) {
		maxItems := int(limit % 12)
		feed, err := Parse(strings.NewReader(string(data)), maxItems)
		if err != nil {
			return
		}
		if feed.Items == nil || len(feed.Items) > maxItems {
			t.Fatalf("Parse returned %d items (nil: %v) for limit %d", len(feed.Items), feed.Items == nil, maxItems)
		}
		checkText(t, "feed title", feed.Title, true)
		for i, it := range feed.Items {
			checkText(t, "item title", it.Title, false)
			if it.Link != "" {
				u, err := url.Parse(it.Link)
				if err != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
					t.Fatalf("item %d link %q is not an absolute http(s) URL", i, it.Link)
				}
			}
			if !it.Published.IsZero() && it.Published.Location().String() != "UTC" {
				t.Fatalf("item %d published %v is not in UTC", i, it.Published)
			}
		}
		// The snapshot with the headlines is sent as JSON; a time that JSON
		// cannot write would fail the whole snapshot.
		if _, err := json.Marshal(feed); err != nil {
			t.Fatalf("feed cannot be encoded as JSON: %v", err)
		}
	})
}

// checkText asserts the rules of PlainText: valid UTF-8 on one short line.
func checkText(t *testing.T, what, s string, mayBeEmpty bool) {
	t.Helper()
	if !utf8.ValidString(s) {
		t.Fatalf("%s %q is not valid UTF-8", what, s)
	}
	if s == "" && !mayBeEmpty {
		t.Fatalf("%s is empty", what)
	}
	if s != strings.TrimSpace(s) || strings.ContainsAny(s, "\r\n\t") {
		t.Fatalf("%s %q is not one trimmed line", what, s)
	}
	if utf8.RuneCountInString(s) > maxTitleRunes {
		t.Fatalf("%s has %d runes, want at most %d", what, utf8.RuneCountInString(s), maxTitleRunes)
	}
}
