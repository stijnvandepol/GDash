package traffic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testdata/incidents.json is the ANWB's answer of 2026-10-13 10:40, cut
// down to a few roads and without the jam shapes.
func recorded(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/incidents.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func recordedReport(t *testing.T) Report {
	t.Helper()
	report, err := decode(recorded(t))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return report
}

// recordedAt is when the answer was recorded.
var recordedAt = time.Date(2026, 10, 13, 8, 40, 0, 0, time.UTC)

// born is a home in Born, next to the A2 in Limburg.
var born = &Point{Latitude: 51.03, Longitude: 5.81}

func TestDecodeReadsUnitsAsTheAnwbSendsThem(t *testing.T) {
	report := recordedReport(t)
	i := slices.IndexFunc(report.Jams, func(in Incident) bool { return in.From == "Born" })
	if i < 0 {
		t.Fatalf("no jam from Born in %+v", report.Jams)
	}
	jam := report.Jams[i]
	// 5400 metres and 660 seconds.
	if jam.Road != "A2" || jam.To != "Urmond" || jam.LengthKm != 5.4 || jam.DelayMinutes != 11 {
		t.Errorf("jam = %+v, want A2 Born to Urmond, 5.4 km, 11 minutes", jam)
	}
	if jam.Reason != "Stilstaand verkeer. Kapotte auto. De rechterrijstrook is dicht." {
		t.Errorf("reason = %q, want the ANWB's own Dutch text", jam.Reason)
	}
	// The totals are in kilometres and minutes already.
	if report.Totals != (Totals{Jams: 15, LengthKm: 41, DelayMinutes: 54}) {
		t.Errorf("totals = %+v", report.Totals)
	}
	if len(report.SpeedChecks) != 1 || report.SpeedChecks[0].Hectometre != 236.1 || report.SpeedChecks[0].Reason != "Bij hectometerpaal 236.1." {
		t.Errorf("speed checks = %+v", report.SpeedChecks)
	}
}

func TestDecodeMarksClosuresAndKeepsTheirTimes(t *testing.T) {
	report := recordedReport(t)
	closed := 0
	for _, in := range report.Roadworks {
		if in.Closed {
			closed++
			if in.Start == nil || in.Stop == nil {
				t.Errorf("closure without its times: %+v", in)
			}
		}
	}
	if closed == 0 {
		t.Errorf("no closures in %+v", report.Roadworks)
	}
	// A closed road in the jam list has no length or delay.
	i := slices.IndexFunc(report.Jams, func(in Incident) bool { return in.Road == "N3" })
	if i < 0 || !report.Jams[i].Closed || report.Jams[i].LengthKm != 0 {
		t.Errorf("N3 closure = %+v", report.Jams)
	}
}

func TestDecodeToleratesMissingAndOddFields(t *testing.T) {
	body := `{"roads":[{"road":"a2","segments":[{"jams":[
		{"from":"Born","to":"Urmond","distance":"far","delay":60},
		{"from":"Born","to":"Urmond","distance":2000,"delay":-5,"fromLoc":{"lat":51.0,"lon":5.8},"newField":{"x":1}},
		{"road":"","from":"\u0007Weert\n","distance":1e12}
	]}]}],"totals":{}}`
	report, err := decode([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The first item has a distance of the wrong type and is left out; the
	// others take the road of their segment, in capitals.
	if len(report.Jams) != 2 {
		t.Fatalf("jams = %+v, want 2", report.Jams)
	}
	if got := report.Jams[0]; got.Road != "A2" || got.LengthKm != 2 || got.DelayMinutes != 0 || len(got.points) != 1 {
		t.Errorf("jam = %+v", got)
	}
	if got := report.Jams[1]; got.From != "Weert" || got.LengthKm != 0 {
		t.Errorf("jam with odd text and length = %+v", got)
	}
}

func TestDecodeRefusesAnAnswerWithoutRoads(t *testing.T) {
	for _, body := range []string{`{"success":false}`, `not json`, `{"roads":null}`} {
		if _, err := decode([]byte(body)); err == nil {
			t.Errorf("decode(%s) gave no error", body)
		}
	}
	report, err := decode([]byte(`{"roads":[]}`))
	if err != nil || len(report.Jams) != 0 {
		t.Errorf("an empty list is a quiet day: %+v, %v", report, err)
	}
}

func TestSelectByRoads(t *testing.T) {
	report := recordedReport(t)
	got := Select(Sources{Report: &report}, Query{Roads: []string{"A2", "A50"}}, nil, recordedAt)
	if !got.Incidents || got.Totals == nil || got.Totals.Jams != 15 {
		t.Errorf("result = %+v", got)
	}
	var jams []string
	for _, j := range got.Jams {
		jams = append(jams, j.Road+" "+j.From)
	}
	// Longest delay first: 11 minutes from Born, 10 from Batadorp, 3 on the A50.
	if want := []string{"A2 Born", "A2 knp. Batadorp", "A50 knp. Ewijk"}; !slices.Equal(jams, want) {
		t.Errorf("jams = %v, want %v", jams, want)
	}
	for _, in := range got.Roadworks {
		if in.Road != "A2" {
			t.Errorf("roadworks on %s for a widget that watches the A2 and A50", in.Road)
		}
		if in.Start != nil && in.Start.After(recordedAt.Add(roadworksAhead)) {
			t.Errorf("roadworks that start after tomorrow: %+v", in)
		}
	}
	if len(got.SpeedChecks) != 1 {
		t.Errorf("speed checks = %+v", got.SpeedChecks)
	}
}

func TestSelectByDistanceFromHome(t *testing.T) {
	report := recordedReport(t)
	got := Select(Sources{Report: &report}, Query{RadiusKm: 10}, born, recordedAt)
	if len(got.Jams) != 1 || got.Jams[0].From != "Born" {
		t.Errorf("jams within 10 km of Born = %+v", got.Jams)
	}
	if len(got.SpeedChecks) != 1 {
		t.Errorf("the speed check at hm 236.1 is 2 km away: %+v", got.SpeedChecks)
	}
	far := Select(Sources{Report: &report}, Query{RadiusKm: 10}, &Point{Latitude: 53.2, Longitude: 6.56}, recordedAt)
	if len(far.Jams) != 0 || len(far.SpeedChecks) != 0 {
		t.Errorf("Groningen has nothing within 10 km: %+v", far)
	}
	noHome := Select(Sources{Report: &report}, Query{RadiusKm: 100}, nil, recordedAt)
	if len(noHome.Jams) != 0 {
		t.Errorf("without a home nothing is near: %+v", noHome.Jams)
	}
}

func TestSelectDropsFinishedRoadworks(t *testing.T) {
	report := recordedReport(t)
	later := recordedAt.Add(10 * 24 * time.Hour)
	for _, in := range Select(Sources{Report: &report}, Query{Roads: []string{"A2"}}, nil, later).Roadworks {
		if in.Stop != nil && in.Stop.Before(later) {
			t.Errorf("finished roadworks shown: %+v", in)
		}
	}
}

func TestSelectCameras(t *testing.T) {
	cameras := []Camera{
		{Kind: CameraFixed, Roads: []string{"A2"}, Name: "Flitspaal hm 236,4", Hectometre: 236.4, Latitude: 51.02, Longitude: 5.80},
		{Kind: CameraSection, Roads: []string{"A2"}, Latitude: 51.6, Longitude: 5.3},
		{Kind: CameraFixed, Latitude: 51.04, Longitude: 5.83},
		{Kind: CameraFixed, Roads: []string{"N279"}, Latitude: 51.6, Longitude: 5.6},
	}
	byRoad := Select(Sources{Cameras: cameras, CamerasLoaded: true}, Query{Roads: []string{"A2"}}, nil, recordedAt)
	if byRoad.Incidents || byRoad.CameraCount != 2 || byRoad.Cameras[0].Road != "A2" {
		t.Errorf("cameras on the A2 = %+v", byRoad)
	}
	near := Select(Sources{Cameras: cameras, CamerasLoaded: true}, Query{RadiusKm: 5}, born, recordedAt)
	if near.CameraCount != 2 || near.Cameras[0].DistanceKm > near.Cameras[1].DistanceKm {
		t.Errorf("cameras near Born, nearest first = %+v", near.Cameras)
	}
	notLoaded := Select(Sources{}, Query{Roads: []string{"A2"}}, nil, recordedAt)
	if notLoaded.Cameras != nil {
		t.Errorf("cameras before the list is loaded = %+v", notLoaded.Cameras)
	}
}

func TestKeysMatchTheWidget(t *testing.T) {
	// web/src/widgets/traffic.ts builds the same keys.
	if got := (Query{Roads: []string{"A2", "A50", "N279"}}).Key(); got != "roads:A2,A50,N279" {
		t.Errorf("key = %q", got)
	}
	if got := (Query{RadiusKm: 25}).Key(); got != "near:25" {
		t.Errorf("key = %q", got)
	}
}

func TestNormalizeRoads(t *testing.T) {
	roads, ok := NormalizeRoads([]string{" a50", "A2", "n279", "A2"})
	if !ok || !slices.Equal(roads, []string{"A2", "A50", "N279"}) {
		t.Errorf("roads = %v, %v", roads, ok)
	}
	for _, bad := range [][]string{{"E25"}, {"A0"}, {"A1234"}, {"A 2"}, {""}, strings.Fields("A1 A2 A3 A4 A5 A6 A7 A8 A9 A10 A11")} {
		if _, ok := NormalizeRoads(bad); ok {
			t.Errorf("NormalizeRoads(%q) accepted", bad)
		}
	}
}

// serve answers like the ANWB with the statuses in turn, and counts.
func serve(t *testing.T, statuses ...int) (Client, *atomic.Int32, *http.Header) {
	t.Helper()
	var count atomic.Int32
	var headers http.Header
	body := recorded(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(count.Add(1))
		headers = r.Header.Clone()
		status := statuses[min(n, len(statuses))-1]
		w.WriteHeader(status)
		if status == http.StatusOK {
			w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 5 * time.Second}, UserAgent: "GDash/test"}, &count, &headers
}

func TestClientNamesGDash(t *testing.T) {
	client, count, headers := serve(t, http.StatusOK)
	if _, err := client.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if count.Load() != 1 || headers.Get("User-Agent") != "GDash/test" || headers.Get("Accept") != "application/json" {
		t.Errorf("requests %d, headers %v", count.Load(), headers)
	}
}

// clock is a time that tests move.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func TestGuardSharesOneRequestPerInterval(t *testing.T) {
	client, count, _ := serve(t, http.StatusOK)
	c := &clock{now: recordedAt}
	g := &Guard{Client: client, Now: c.Now}
	for range 3 {
		if _, err := g.Fetch(context.Background()); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		c.now = c.now.Add(time.Minute)
	}
	if count.Load() != 1 {
		t.Errorf("%d requests within 3 minutes, want 1", count.Load())
	}
	c.now = recordedAt.Add(MinInterval)
	if _, err := g.Fetch(context.Background()); err != nil || count.Load() != 2 {
		t.Errorf("after the interval: %v, %d requests", err, count.Load())
	}
	if g.Status().State != StateOK {
		t.Errorf("status = %+v", g.Status())
	}
}

func TestGuardBacksOffAfterFailures(t *testing.T) {
	client, count, _ := serve(t, http.StatusBadGateway, http.StatusBadGateway, http.StatusOK)
	c := &clock{now: recordedAt}
	g := &Guard{Client: client, Now: c.Now}
	if _, err := g.Fetch(context.Background()); err == nil {
		t.Fatal("a 502 gave no error")
	}
	// The wait doubles: 6 minutes after the first failure.
	c.now = recordedAt.Add(5 * time.Minute)
	if _, err := g.Fetch(context.Background()); err != ErrWaiting || count.Load() != 1 {
		t.Errorf("within the wait: %v, %d requests", err, count.Load())
	}
	st := g.Status()
	if st.State != StateFailing || st.HTTPStatus != http.StatusBadGateway || !st.NextAttemptAt.Equal(recordedAt.Add(6*time.Minute)) {
		t.Errorf("status = %+v", st)
	}
	c.now = recordedAt.Add(6 * time.Minute)
	g.Fetch(context.Background())
	// Then 12 minutes after the second.
	c.now = c.now.Add(12 * time.Minute)
	if _, err := g.Fetch(context.Background()); err != nil || count.Load() != 3 {
		t.Errorf("after the second wait: %v, %d requests", err, count.Load())
	}
}

func TestGuardStopsAfterRepeatedRefusals(t *testing.T) {
	client, count, _ := serve(t, http.StatusForbidden)
	c := &clock{now: recordedAt}
	g := &Guard{Client: client, Now: c.Now}
	for range 5 {
		g.Fetch(context.Background())
		c.now = c.now.Add(time.Hour)
	}
	if count.Load() != stopAfterRefusals {
		t.Errorf("%d requests, want %d before stopping", count.Load(), stopAfterRefusals)
	}
	if _, err := g.Fetch(context.Background()); err != ErrStopped {
		t.Errorf("after stopping: %v", err)
	}
	if st := g.Status(); st.State != StateStopped || st.HTTPStatus != http.StatusForbidden {
		t.Errorf("status = %+v", st)
	}
	g.Reset()
	g.Fetch(context.Background())
	if count.Load() != stopAfterRefusals+1 {
		t.Errorf("Reset did not let GDash ask again: %d requests", count.Load())
	}
}

func TestGuardKeepsAskingAfterTooManyRequests(t *testing.T) {
	client, count, _ := serve(t, http.StatusTooManyRequests)
	c := &clock{now: recordedAt}
	g := &Guard{Client: client, Now: c.Now}
	for range 5 {
		g.Fetch(context.Background())
		c.now = c.now.Add(time.Hour)
	}
	if count.Load() != 5 || g.Status().State != StateFailing {
		t.Errorf("429 is a reason to wait, not to stop: %d requests, %+v", count.Load(), g.Status())
	}
}
