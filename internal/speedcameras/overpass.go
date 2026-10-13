package speedcameras

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/stijnvandepol/gdash/internal/geo"
	"github.com/stijnvandepol/gdash/internal/traffic"
)

// query asks for every fixed camera node and enforcement relation in the
// Netherlands, each followed by the roads with a ref within 30 metres of it
// (of its member nodes, for a relation), cut down to their ref tags.
const query = `[out:json][timeout:90];
area["ISO3166-1"="NL"][admin_level=2]->.nl;
(
  node["highway"="speed_camera"](area.nl);
  relation["type"="enforcement"]["enforcement"~"^(maxspeed|average_speed)$"](area.nl);
)->.cams;
foreach.cams->.c(
  .c out center tags;
  (node.c; node(r.c);)->.at;
  way(around.at:30)["highway"]["ref"];
  convert road ::id=id(),ref=t["ref"];
  out;
);`

const (
	// maxAnswerBytes bounds what is read: the country is a few thousand
	// elements of a few hundred bytes.
	maxAnswerBytes = 16 << 20
	// duplicateKm is how close a camera relation may be to a camera node
	// before both are taken to be the same camera.
	duplicateKm  = 0.1
	maxNameRunes = 80
)

// Client asks the Overpass API.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// UserAgent names GDash and its version, as the usage policy asks.
	UserAgent string
}

// Fetch returns every camera in the country.
func (c Client) Fetch(ctx context.Context) ([]traffic.Camera, error) {
	form := url.Values{"data": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build overpass request: %w", err)
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request overpass: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read overpass answer: %w", err)
	}
	if len(body) > maxAnswerBytes {
		return nil, fmt.Errorf("overpass answer is larger than %d bytes", maxAnswerBytes)
	}
	return decode(body)
}

type element struct {
	Type   string  `json:"type"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Center *struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"center"`
	Tags map[string]string `json:"tags"`
}

// found is a camera while the answer is read.
type found struct {
	traffic.Camera
	// relation is true for an enforcement relation, false for a node.
	relation bool
	// refs are the ref tags of the roads next to it.
	refs []string
}

// decode reads an Overpass answer: each camera, followed by the roads near
// it. An answer without an element list is an error, as is one that says
// the query did not finish (Overpass then answers 200 with a remark).
func decode(body []byte) ([]traffic.Camera, error) {
	var answer struct {
		Elements *[]json.RawMessage `json:"elements"`
		Remark   string             `json:"remark"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("decode overpass answer: %w", err)
	}
	if answer.Elements == nil {
		return nil, errors.New("overpass answer has no element list")
	}
	if strings.Contains(strings.ToLower(answer.Remark), "error") {
		return nil, fmt.Errorf("overpass did not finish the query: %.200s", answer.Remark)
	}
	var cameras []found
	// current is the camera that the roads which follow belong to, or -1
	// after one that was left out.
	current := -1
	for _, raw := range *answer.Elements {
		var e element
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		switch e.Type {
		case "node", "relation":
			current = -1
			if camera, ok := e.camera(); ok {
				cameras = append(cameras, camera)
				current = len(cameras) - 1
			}
		case "road":
			if current >= 0 && e.Tags["ref"] != "" {
				cameras[current].refs = append(cameras[current].refs, e.Tags["ref"])
			}
		}
	}
	for i := range cameras {
		if len(cameras[i].Roads) == 0 {
			cameras[i].Roads = roadsFromRefs(cameras[i].refs)
		}
	}
	return withoutDuplicates(cameras), nil
}

// camera reads a camera node or an enforcement relation, or false for one
// without a place.
func (e element) camera() (found, bool) {
	lat, lon := e.Lat, e.Lon
	if e.Center != nil {
		lat, lon = e.Center.Lat, e.Center.Lon
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
		return found{}, false
	}
	name := strings.TrimSpace(e.Tags["name"])
	if runes := []rune(name); len(runes) > maxNameRunes {
		name = string(runes[:maxNameRunes])
	}
	c := traffic.Camera{
		Kind:       traffic.CameraFixed,
		Name:       name,
		MaxSpeed:   maxSpeed(e.Tags["maxspeed"]),
		Hectometre: hectometre(name),
		Latitude:   lat,
		Longitude:  lon,
	}
	relation := e.Type == "relation"
	if relation && e.Tags["enforcement"] == "average_speed" {
		c.Kind = traffic.CameraSection
	}
	// A relation often names its road ("Trajectcontrole A2"), which says
	// more than the roads that happen to be near its members.
	if relation {
		var named []string
		for _, m := range roadInName.FindAllStringSubmatch(name, -1) {
			named = append(named, m[1])
		}
		c.Roads = roadsFromRefs(named)
	}
	return found{Camera: c, relation: relation}, true
}

// withoutDuplicates drops a fixed-speed enforcement relation that lies
// next to a camera node: mappers often tag both for one camera. Only the
// few relations are compared with the nodes.
func withoutDuplicates(cameras []found) []traffic.Camera {
	out := make([]traffic.Camera, 0, len(cameras))
	for _, c := range cameras {
		if c.relation && c.Kind == traffic.CameraFixed && hasNodeNear(cameras, c.Camera) {
			continue
		}
		out = append(out, c.Camera)
	}
	return out
}

func hasNodeNear(cameras []found, relation traffic.Camera) bool {
	for _, other := range cameras {
		if !other.relation && geo.DistanceKm(relation.Latitude, relation.Longitude, other.Latitude, other.Longitude) < duplicateKm {
			return true
		}
	}
	return false
}
