package departures

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// recordedAt is when testdata/boards.json was recorded from OVapi, a few
// minutes before the first bus in it.
var recordedAt = time.Date(2026, 10, 12, 8, 48, 22, 0, time.UTC)

func recorded(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// serve answers every request with status and body and remembers the last
// request.
func serve(t *testing.T, status int, body []byte) (Client, *http.Request) {
	t.Helper()
	var got http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = *r
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	client := Client{
		BaseURL:   srv.URL,
		HTTP:      &http.Client{Timeout: 5 * time.Second},
		UserAgent: "GDash-test",
		Now:       func() time.Time { return recordedAt },
	}
	return client, &got
}

func TestBoardsAreParsedFilteredAndSorted(t *testing.T) {
	client, req := serve(t, http.StatusOK, recorded(t, "boards.json"))

	got, err := client.Boards(context.Background(), []string{"utrcsw", "utrbus", "zzzzzz"})
	if err != nil {
		t.Fatalf("Boards: %v", err)
	}
	if req.URL.Path != "/stopareacode/utrcsw,utrbus,zzzzzz" {
		t.Errorf("path = %q, want one request for all three stops", req.URL.Path)
	}
	if req.Header.Get("User-Agent") != "GDash-test" {
		t.Errorf("User-Agent = %q", req.Header.Get("User-Agent"))
	}

	// The recording holds nine passes of the bus station: one has passed,
	// two end their journey there. The rest come out soonest first.
	type row struct {
		clock, destination string
		delay              int
	}
	want := []row{
		{"10:54", "Science Park P+R", 0}, // ahead of the timetable: no delay
		{"11:01", "Science Park P+R", 1}, // planned 11:00
		{"11:05", "Vleuterweide", 0},
		{"11:12", "Science Park P+R", 0}, // 2 seconds late is on time
		{"11:12", "Vleuterweide", 19},    // planned 10:53
		{"11:48", "Science Park P+R", 0},
	}
	board := got["utrcsw"]
	if len(board) != len(want) {
		t.Fatalf("utrcsw has %d departures, want %d: %+v", len(board), len(want), board)
	}
	for i, w := range want {
		d := board[i]
		clock := d.Expected.Format("15:04")
		if clock != w.clock || d.Destination != w.destination || d.DelayMinutes != w.delay || d.Line != "28" || d.Cancelled {
			t.Errorf("departure %d = %s %s +%d (%+v), want %+v", i, clock, d.Destination, d.DelayMinutes, d, w)
		}
	}
	// Every pass of the tram stop ends its journey there, and OVapi knows
	// no departures for the made-up code: both are present and empty.
	for _, code := range []string{"utrbus", "zzzzzz"} {
		if list, ok := got[code]; !ok || list == nil || len(list) != 0 {
			t.Errorf("%s = %#v, want an empty list", code, list)
		}
	}
}

func TestTimesAreReadInTheNetherlands(t *testing.T) {
	zone, err := amsterdam()
	if err != nil {
		t.Skipf("no time zone data: %v", err)
	}
	pass := func(planned, expected string) apiPass {
		return apiPass{Line: "28", Destination: "Vleuterweide", Status: "PLANNED", StopType: "INTERMEDIATE", TargetDepartureTime: planned, ExpectedDepartureTime: expected}
	}
	tests := []struct {
		name            string
		planned, expect string
		wantUTC         string
		wantDelay       int
	}{
		{"summer time", "2026-10-12T11:48:00", "2026-10-12T11:48:00", "2026-10-12T09:48:00Z", 0},
		{"winter time", "2026-12-01T11:48:00", "2026-12-01T11:50:30", "2026-12-01T10:50:30Z", 2},
		{"across midnight", "2026-10-12T23:55:00", "2026-10-13T00:03:00", "2026-10-12T22:03:00Z", 8},
		// The clocks go back at 03:00: 01:50 is still summer time and 03:10
		// is winter time, so 140 real minutes pass, not 80.
		{"the night the clocks go back", "2026-10-25T01:50:00", "2026-10-25T03:10:00", "2026-10-25T02:10:00Z", 140},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// now is given in UTC: the server's own zone must not matter.
			d, ok, err := pass(tt.planned, tt.expect).departure(zone, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
			if err != nil || !ok {
				t.Fatalf("pass was dropped: ok %v, err %v", ok, err)
			}
			if got := d.Expected.UTC().Format(time.RFC3339); got != tt.wantUTC {
				t.Errorf("Expected = %s, want %s", got, tt.wantUTC)
			}
			if d.DelayMinutes != tt.wantDelay {
				t.Errorf("DelayMinutes = %d, want %d", d.DelayMinutes, tt.wantDelay)
			}
		})
	}
}

func TestDeparturesLeaveTheBoardWithTheirMinute(t *testing.T) {
	zone, err := amsterdam()
	if err != nil {
		t.Skipf("no time zone data: %v", err)
	}
	p := apiPass{Line: "28", Status: "DRIVING", StopType: "INTERMEDIATE", TargetDepartureTime: "2026-10-12T10:54:00", ExpectedDepartureTime: "2026-10-12T10:54:36"}
	at := func(clock string) time.Time {
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", "2026-10-12 "+clock, zone)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}
	if _, ok, _ := p.departure(zone, at("10:54:50")); !ok {
		t.Error("a bus due at 10:54:36 was dropped at 10:54:50, in the same minute")
	}
	if _, ok, _ := p.departure(zone, at("10:55:00")); ok {
		t.Error("a bus due at 10:54 was still listed at 10:55")
	}
}

func TestPassesThatAreNotDeparturesAreDropped(t *testing.T) {
	zone, err := amsterdam()
	if err != nil {
		t.Skipf("no time zone data: %v", err)
	}
	base := apiPass{Line: "5", Destination: "Centrum", Status: "PLANNED", StopType: "FIRST", TargetDepartureTime: "2026-10-12T12:00:00", ExpectedDepartureTime: "2026-10-12T12:00:00"}
	if _, ok, err := base.departure(zone, recordedAt); !ok || err != nil {
		t.Fatalf("the base pass is not a departure: %v", err)
	}
	changes := map[string]func(*apiPass){
		"passed":          func(p *apiPass) { p.Status = "PASSED" },
		"journey ends":    func(p *apiPass) { p.StopType = "LAST" },
		"no line":         func(p *apiPass) { p.Line = "" },
		"no planned time": func(p *apiPass) { p.TargetDepartureTime = "" },
		"no real time":    func(p *apiPass) { p.ExpectedDepartureTime = "" },
	}
	for name, change := range changes {
		p := base
		change(&p)
		if _, ok, err := p.departure(zone, recordedAt); ok || err != nil {
			t.Errorf("%s: kept %v, error %v; want a quiet drop", name, ok, err)
		}
	}
}

func TestAChangedTimeFormatFailsTheRefresh(t *testing.T) {
	body := `{"abc":{"1":{"Passes":{"x":{"LinePublicNumber":"28","DestinationName50":"Vleuterweide","TripStopStatus":"PLANNED",
		"JourneyStopType":"FIRST","TargetDepartureTime":"2026-10-12T12:00:00+02:00","ExpectedDepartureTime":"2026-10-12T12:00:00+02:00"}}}}}`
	_, err := parseBoards([]byte(body), []string{"abc"}, recordedAt)
	if err == nil || !strings.Contains(err.Error(), "stop abc, pass x") {
		t.Fatalf("err = %v, want the stop and pass named so the log says what changed", err)
	}
}

func TestCancelledDeparturesStayOnTheBoard(t *testing.T) {
	body := `{"abc":{"1":{"Passes":{"x":{"LinePublicNumber":"28","DestinationName50":"Vleuterweide","TripStopStatus":"CANCEL",
		"JourneyStopType":"INTERMEDIATE","TargetDepartureTime":"2026-10-12T12:00:00","ExpectedDepartureTime":"2026-10-12T12:00:00"}}}}}`
	got, err := parseBoards([]byte(body), []string{"abc"}, recordedAt)
	if err != nil {
		t.Fatal(err)
	}
	if list := got["abc"]; len(list) != 1 || !list[0].Cancelled || list[0].DelayMinutes != 0 {
		t.Fatalf("abc = %+v, want one cancelled departure", list)
	}
}

func TestOnlyTheNextDeparturesAreKept(t *testing.T) {
	var passes []string
	for i := range MaxPerStop + 5 {
		// Written in reverse, so the cut depends on sorting.
		minute := MaxPerStop + 4 - i
		passes = append(passes, fmt.Sprintf(`"p%d":{"LinePublicNumber":"%d","DestinationName50":"X","TripStopStatus":"PLANNED","JourneyStopType":"FIRST",
			"TargetDepartureTime":"2026-10-12T11:%02d:00","ExpectedDepartureTime":"2026-10-12T11:%02d:00"}`, i, i, minute, minute))
	}
	body := `{"abc":{"1":{"Passes":{` + strings.Join(passes, ",") + `}}}}`
	got, err := parseBoards([]byte(body), []string{"abc"}, recordedAt)
	if err != nil {
		t.Fatal(err)
	}
	list := got["abc"]
	if len(list) != MaxPerStop || list[0].Expected.Format("15:04") != "11:00" || list[len(list)-1].Expected.Format("15:04") != "11:19" {
		t.Fatalf("kept %d departures from %v to %v, want the first %d", len(list), list[0].Expected, list[len(list)-1].Expected, MaxPerStop)
	}
}

func TestBoardsFailCleanly(t *testing.T) {
	t.Run("server error", func(t *testing.T) {
		client, _ := serve(t, http.StatusServiceUnavailable, []byte("down"))
		if _, err := client.Boards(context.Background(), []string{"utrcsw"}); err == nil || !strings.Contains(err.Error(), "503") {
			t.Fatalf("err = %v, want the status", err)
		}
	})
	t.Run("not json", func(t *testing.T) {
		client, _ := serve(t, http.StatusOK, []byte("<html>maintenance</html>"))
		if _, err := client.Boards(context.Background(), []string{"utrcsw"}); err == nil {
			t.Fatal("an HTML answer was accepted")
		}
	})
	t.Run("invalid code is never sent", func(t *testing.T) {
		client, req := serve(t, http.StatusOK, []byte(`{}`))
		for _, code := range []string{"", "a/../b", "utrcsw,x", "bus stop", "a?b=c"} {
			if _, err := client.Boards(context.Background(), []string{code}); err == nil {
				t.Errorf("code %q was accepted", code)
			}
		}
		if req.URL != nil {
			t.Fatalf("a request was made: %s", req.URL)
		}
	})
	t.Run("no stops, no request", func(t *testing.T) {
		client, req := serve(t, http.StatusOK, []byte(`{}`))
		got, err := client.Boards(context.Background(), nil)
		if err != nil || len(got) != 0 || req.URL != nil {
			t.Fatalf("Boards(nil) = %v, %v; request %v", got, err, req.URL)
		}
	})
}

func TestStopsAreListedWithTheirTown(t *testing.T) {
	client, req := serve(t, http.StatusOK, recorded(t, "stopareas.json"))

	stops, err := client.Stops(context.Background())
	if err != nil {
		t.Fatalf("Stops: %v", err)
	}
	if req.URL.Path != "/stopareacode" {
		t.Errorf("path = %q", req.URL.Path)
	}
	names := map[string]string{}
	for _, s := range stops {
		names[s.Code] = s.Name
	}
	if len(stops) != 10 {
		t.Errorf("%d stops, want the 12 recorded minus the 2 with a space or & in the code: %v", len(stops), names)
	}
	// The name already starts with the town, or gets it.
	if names["utrcsw"] != "Utrecht, CS Jaarbeurszijde" || names["05003"] != "Amsterdam, Centraal Station" || names["asdcs"] != "Amsterdam, Centraal Station" {
		t.Errorf("names = %v", names)
	}
	if _, ok := names["Bd CS"]; ok {
		t.Error("a code with a space was listed")
	}
	for i := 1; i < len(stops); i++ {
		if stops[i-1].Name > stops[i].Name {
			t.Fatalf("stops are not sorted by name: %q before %q", stops[i-1].Name, stops[i].Name)
		}
	}
}

func TestEmptyStopListIsAnError(t *testing.T) {
	client, _ := serve(t, http.StatusOK, []byte(`{}`))
	if _, err := client.Stops(context.Background()); err == nil {
		t.Fatal("an empty list was accepted; the editor would search nothing")
	}
}
