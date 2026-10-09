package cast

import "encoding/json"

// DashCast is a public receiver (https://madmod.github.io/dashcast/receiver.html)
// that opens any URL on a Cast device. GDash uses it until a user registers an own
// receiver: it needs no Cast developer account, and on real hardware it
// kept a GDash display page on a Google TV.
const (
	DashCastAppID     = "84912283"
	DashCastNamespace = "urn:x-cast:com.madmod.dashcast"
)

// DashCastLoad returns the message that makes DashCast open url. "force"
// replaces the receiver page with url instead of loading it in an iframe;
// an iframe would be blocked as mixed content, because DashCast runs on
// HTTPS and GDash usually on plain HTTP in the home network.
func DashCastLoad(url string) []byte {
	msg, _ := json.Marshal(map[string]any{"url": url, "force": true, "reload": false, "reload_time": 0})
	return msg
}
