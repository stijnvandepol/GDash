package cast

import (
	"net"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func mustName(t *testing.T, s string) dnsmessage.Name {
	t.Helper()
	n, err := dnsmessage.NewName(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

type castAnnouncement struct {
	instance, host string
	ip             [4]byte
	port           uint16
	txt            []string
	withA          bool
}

func buildResponse(t *testing.T, a castAnnouncement) []byte {
	t.Helper()
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{Response: true, Authoritative: true})
	if err := b.StartAnswers(); err != nil {
		t.Fatal(err)
	}
	instance := mustName(t, a.instance)
	host := mustName(t, a.host)
	hdr := func(name dnsmessage.Name, typ dnsmessage.Type) dnsmessage.ResourceHeader {
		return dnsmessage.ResourceHeader{Name: name, Type: typ, Class: dnsmessage.ClassINET, TTL: 120}
	}
	if err := b.PTRResource(hdr(mustName(t, castService), dnsmessage.TypePTR), dnsmessage.PTRResource{PTR: instance}); err != nil {
		t.Fatal(err)
	}
	if err := b.StartAdditionals(); err != nil {
		t.Fatal(err)
	}
	if err := b.SRVResource(hdr(instance, dnsmessage.TypeSRV), dnsmessage.SRVResource{Target: host, Port: a.port}); err != nil {
		t.Fatal(err)
	}
	if err := b.TXTResource(hdr(instance, dnsmessage.TypeTXT), dnsmessage.TXTResource{TXT: a.txt}); err != nil {
		t.Fatal(err)
	}
	if a.withA {
		if err := b.AResource(hdr(host, dnsmessage.TypeA), dnsmessage.AResource{A: a.ip}); err != nil {
			t.Fatal(err)
		}
	}
	msg, err := b.Finish()
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestCollectorParsesCastAnnouncement(t *testing.T) {
	acc := newCollector()
	packet := buildResponse(t, castAnnouncement{
		instance: "Google-TV-Streamer-abc._googlecast._tcp.local.",
		host:     "abc.local.",
		ip:       [4]byte{192, 168, 1, 40},
		port:     8009,
		txt:      []string{"id=abc123", "fn=Keuken TV", "md=Google TV Streamer"},
		withA:    true,
	})
	if err := acc.add(packet, net.IPv4(192, 168, 1, 99)); err != nil {
		t.Fatalf("add: %v", err)
	}
	got := acc.devices()
	want := []Device{{ID: "abc123", Name: "Keuken TV", Model: "Google TV Streamer", Addr: "192.168.1.40:8009"}}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("devices = %+v, want %+v", got, want)
	}
}

func TestCollectorFallsBackToSourceAddress(t *testing.T) {
	acc := newCollector()
	packet := buildResponse(t, castAnnouncement{
		instance: "Chromecast-def._googlecast._tcp.local.",
		host:     "def.local.",
		port:     8009,
		txt:      []string{"id=def456", "fn=Woonkamer"},
	})
	if err := acc.add(packet, net.IPv4(192, 168, 1, 50)); err != nil {
		t.Fatalf("add: %v", err)
	}
	got := acc.devices()
	if len(got) != 1 || got[0].Addr != "192.168.1.50:8009" {
		t.Fatalf("devices = %+v, want address from packet source", got)
	}
}

func TestCollectorDeduplicatesRepeatedAnswers(t *testing.T) {
	acc := newCollector()
	a := castAnnouncement{
		instance: "Chromecast-def._googlecast._tcp.local.",
		host:     "def.local.",
		ip:       [4]byte{192, 168, 1, 50},
		port:     8009,
		txt:      []string{"id=def456", "fn=Woonkamer"},
		withA:    true,
	}
	for range 3 {
		if err := acc.add(buildResponse(t, a), nil); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if got := acc.devices(); len(got) != 1 {
		t.Fatalf("devices = %+v, want exactly one", got)
	}
}

func TestCollectorSkipsInstanceWithoutID(t *testing.T) {
	acc := newCollector()
	packet := buildResponse(t, castAnnouncement{
		instance: "Odd._googlecast._tcp.local.",
		host:     "odd.local.",
		ip:       [4]byte{10, 0, 0, 2},
		port:     8009,
		txt:      []string{"fn=No ID"},
		withA:    true,
	})
	if err := acc.add(packet, nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := acc.devices(); len(got) != 0 {
		t.Fatalf("devices = %+v, want none", got)
	}
}

func TestCollectorRejectsGarbage(t *testing.T) {
	if err := newCollector().add([]byte{0x01, 0x02}, nil); err == nil {
		t.Fatal("add accepted a malformed packet")
	}
}

func TestBuildQueryAsksForCastService(t *testing.T) {
	q, err := buildQuery()
	if err != nil {
		t.Fatal(err)
	}
	var p dnsmessage.Parser
	if _, err := p.Start(q); err != nil {
		t.Fatal(err)
	}
	question, err := p.Question()
	if err != nil {
		t.Fatal(err)
	}
	if question.Name.String() != castService || question.Type != dnsmessage.TypePTR {
		t.Fatalf("question = %v, want PTR %s", question, castService)
	}
}
