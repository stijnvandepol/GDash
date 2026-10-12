package fuel

import (
	"math"
	"os"
	"strings"
	"testing"
)

// oss is the centre of Oss; testdata/oss.json holds the 19 stations the ANWB
// listed in the box around it, recorded on 2026-10-12.
var oss = Area{Latitude: 51.76, Longitude: 5.52, RadiusKm: 3}

func recorded(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/oss.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func recordedStations(t *testing.T) []poi {
	t.Helper()
	stations, err := decode(recorded(t))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return stations
}

func names(stations []Station) []string {
	out := make([]string, len(stations))
	for i, s := range stations {
		out[i] = s.Name
	}
	return out
}

func TestRecordedAnswerIsDecoded(t *testing.T) {
	if got := len(recordedStations(t)); got != 19 {
		t.Fatalf("decoded %d stations, want the 19 that were recorded", got)
	}
}

func TestCheapestStationsComeFirstAndNearerWinsATie(t *testing.T) {
	got := pick(recordedStations(t), oss, "EURO95")

	if len(got) != MaxStations {
		t.Fatalf("got %d stations, want the list capped at %d", len(got), MaxStations)
	}
	first := Station{Name: "de Kock (Self service), Oss", Price: 2.364, DistanceKm: 1.7}
	if got[0] != first {
		t.Errorf("cheapest = %+v, want %+v", got[0], first)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Price < got[i-1].Price {
			t.Fatalf("%s (%.3f) is listed after %s (%.3f)", got[i].Name, got[i].Price, got[i-1].Name, got[i-1].Price)
		}
	}
	// Two stations ask 2.429; the one 0.7 km away goes before the one 2.6 km away.
	var tied []string
	for _, s := range got {
		if s.Price == 2.429 {
			tied = append(tied, s.Name)
		}
	}
	if strings.Join(tied, "|") != "Tango Dr Saal v Zwanenbergsingel, Oss|Tango Singel 1940-1945, Oss" {
		t.Errorf("stations at 2.429 = %v, want the nearer first", tied)
	}
}

func TestOnlyStationsInsideTheRadiusCount(t *testing.T) {
	all := pick(recordedStations(t), Area{Latitude: 51.76, Longitude: 5.52, RadiusKm: MaxRadiusKm}, "EURO95")
	near := pick(recordedStations(t), oss, "EURO95")
	if !contains(names(all), "Argos, Geffen") || contains(names(near), "Argos, Geffen") {
		t.Fatalf("Argos in Geffen is 4.1 km away: in the 25 km list %v, in the 3 km list %v", contains(names(all), "Argos, Geffen"), contains(names(near), "Argos, Geffen"))
	}
	for _, s := range near {
		if s.DistanceKm > 3 {
			t.Errorf("%s is %.1f km away, outside the 3 km circle", s.Name, s.DistanceKm)
		}
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// A corner of the box is in the box but about 1.27 times the radius away.
func TestACornerOfTheBoxIsNotInTheCircle(t *testing.T) {
	minLat, minLon, maxLat, maxLon := oss.boundingBox()
	corner := poi{Title: "Corner"}
	corner.Coordinates.Latitude = minLat + 0.05*(maxLat-minLat)
	corner.Coordinates.Longitude = minLon + 0.05*(maxLon-minLon)
	corner.Prices = append(corner.Prices, struct {
		FuelType string  `json:"fuelType"`
		Value    float64 `json:"value"`
	}{"EURO95", 2.0})

	if got := pick([]poi{corner}, oss, "EURO95"); len(got) != 0 {
		t.Fatalf("pick = %+v, want the corner left out", got)
	}
}

func TestBoxHoldsTheWholeCircle(t *testing.T) {
	// The circle touches the box at four points; allow for rounding there.
	const edge = 1e-9
	for _, area := range []Area{oss, {Latitude: 53.4, Longitude: 6.8, RadiusKm: 25}, {Latitude: 50.8, Longitude: 5.7, RadiusKm: 1}} {
		minLat, minLon, maxLat, maxLon := area.boundingBox()
		for bearing := 0.0; bearing < 360; bearing += 5 {
			lat, lon := destination(area, bearing)
			if lat < minLat-edge || lat > maxLat+edge || lon < minLon-edge || lon > maxLon+edge {
				t.Fatalf("%+v: the point %.0f degrees away (%.4f, %.4f) is outside the box", area, bearing, lat, lon)
			}
		}
	}
}

// destination is the point radius km from the area's centre towards bearing.
func destination(a Area, bearing float64) (lat, lon float64) {
	rad := math.Pi / 180
	d := float64(a.RadiusKm) / earthRadiusKm
	lat1, lon1, b := a.Latitude*rad, a.Longitude*rad, bearing*rad
	lat2 := math.Asin(math.Sin(lat1)*math.Cos(d) + math.Cos(lat1)*math.Sin(d)*math.Cos(b))
	lon2 := lon1 + math.Atan2(math.Sin(b)*math.Sin(d)*math.Cos(lat1), math.Cos(d)-math.Sin(lat1)*math.Sin(lat2))
	return lat2 / rad, lon2 / rad
}

func TestDistanceIsGreatCircle(t *testing.T) {
	// Amsterdam Centraal to Rotterdam Centraal is about 57 km as the crow flies.
	if got := distanceKm(52.3791, 4.9003, 51.9244, 4.4693); math.Abs(got-57) > 1.5 {
		t.Errorf("Amsterdam to Rotterdam = %.1f km, want about 57", got)
	}
	if got := distanceKm(51.76, 5.52, 51.76, 5.52); got != 0 {
		t.Errorf("distance to itself = %v, want 0", got)
	}
	if got := distanceKm(0, 0, 0, 180); math.Abs(got-math.Pi*earthRadiusKm) > 0.001 {
		t.Errorf("half the earth = %.1f km", got)
	}
}

func TestPricesBelowTheSanityMinimumAreIgnored(t *testing.T) {
	// The recorded answer holds LPG at 0 euro for the self-service station of de Kock.
	lpg := pick(recordedStations(t), oss, "AUTOGAS")
	if len(lpg) != 3 || lpg[0].Price != 0.889 {
		t.Fatalf("LPG within 3 km = %+v, want three stations from 0.889", lpg)
	}
	if contains(names(lpg), "de Kock (Self service), Oss") {
		t.Error("a station that lists 0 euro for LPG was kept")
	}
	for _, price := range []float64{0, 0.05, -1, 10.5, math.NaN()} {
		s := poi{Title: "Test"}
		s.Prices = append(s.Prices, struct {
			FuelType string  `json:"fuelType"`
			Value    float64 `json:"value"`
		}{"EURO95", price})
		if _, ok := s.price("EURO95"); ok {
			t.Errorf("price %v was accepted", price)
		}
	}
}

func TestAStationWithoutTheFuelIsLeftOut(t *testing.T) {
	got := pick(recordedStations(t), oss, "CNG")
	if len(got) != 1 || got[0].Name != "CNG Express, Oss" || got[0].Price != 1.699 {
		t.Fatalf("CNG = %+v, want only CNG Express", got)
	}
	// An empty answer is an empty list, not null: the widget tells "none" from "waiting".
	if none := pick(recordedStations(t), Area{Latitude: 52.5, Longitude: 6, RadiusKm: 1}, "CNG"); none == nil || len(none) != 0 {
		t.Fatalf("pick far away = %#v, want an empty list", none)
	}
}

func TestNamesGetTheirTownOnlyOnce(t *testing.T) {
	tests := []struct{ title, city, want string }{
		{"Shell Hanex", "Oss", "Shell Hanex, Oss"},
		{"TinQ Oss", "Oss", "TinQ Oss"},
		{"tinq oss", "Oss", "tinq oss"},
		{"  Esso  ", "", "Esso"},
		{"", "Oss", ""},
		{strings.Repeat("x", 100), "Oss", strings.Repeat("x", maxNameRunes)},
	}
	for _, tt := range tests {
		s := poi{Title: tt.title}
		s.Address.City = tt.city
		if got := s.label(); got != tt.want {
			t.Errorf("label(%q, %q) = %q, want %q", tt.title, tt.city, got, tt.want)
		}
	}
}

func TestAnswersWithoutAStationListFail(t *testing.T) {
	for name, body := range map[string]string{
		"not json":       "<html>maintenance</html>",
		"no list":        `{"error":"unavailable"}`,
		"null list":      `{"value":null}`,
		"price as text":  `{"value":[{"title":"x","prices":[{"fuelType":"EURO95","value":"2,10"}]}]}`,
		"list is object": `{"value":{}}`,
	} {
		if _, err := decode([]byte(body)); err == nil {
			t.Errorf("%s: decode succeeded, want an error", name)
		}
	}
	if stations, err := decode([]byte(`{"value":[]}`)); err != nil || len(stations) != 0 {
		t.Errorf("an empty list is a real answer: got %v, %v", stations, err)
	}
}

// The keys below are repeated in web/src/widgets/fuel.test.ts: the widget
// builds the same text from its settings.
func TestKeysMatchWhatTheWidgetBuilds(t *testing.T) {
	tests := []struct {
		query Query
		want  string
	}{
		{Query{Area: Area{51.77, 5.52, 5}, Fuel: "EURO95"}, "51.77,5.52,5,EURO95"},
		{Query{Area: Area{51.7654321, 5.5, 10}, Fuel: "DIESEL"}, "51.765,5.5,10,DIESEL"},
		{Query{Area: Area{-0.0001, -33.8688, 1}, Fuel: "CNG"}, "0,-33.869,1,CNG"},
		{Query{Area: Area{52, 4, 25}, Fuel: "AUTOGAS"}, "52,4,25,AUTOGAS"},
		{Query{Here: true, Area: Area{51.77, 5.52, 5}, Fuel: "EURO98"}, "here,5,EURO98"},
	}
	for _, tt := range tests {
		if got := tt.query.Key(); got != tt.want {
			t.Errorf("Key(%+v) = %q, want %q", tt.query, got, tt.want)
		}
	}
}
