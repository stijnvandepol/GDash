package api

import (
	"net/http"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/traffic"
)

// TrafficGuard is the part of the ANWB request guard the web interface
// uses: how the requests go, and starting again after a stop.
type TrafficGuard interface {
	Status() traffic.Status
	Reset()
}

// trafficView is GET /api/traffic: the switch, how the requests to the
// ANWB go, and the data status of the widgets and the camera list.
type trafficView struct {
	settings.TrafficInfo
	ANWB    traffic.Status `json:"anwb"`
	Data    *sourceStatus  `json:"data,omitempty"`
	Cameras *sourceStatus  `json:"cameras,omitempty"`
}

// getTraffic handles GET /api/traffic.
func (s *Server) getTraffic(w http.ResponseWriter, r *http.Request) {
	info, err := s.Settings.TrafficInfo(r.Context())
	if err != nil {
		s.internalError(w, "read traffic info", err)
		return
	}
	entries := s.SourceStatus()
	writeJSON(w, http.StatusOK, trafficView{
		TrafficInfo: info,
		ANWB:        s.Traffic.Status(),
		Data:        statusOf(entries, display.TrafficSource),
		Cameras:     statusOf(entries, display.CamerasSource),
	})
}

// putTraffic handles PUT /api/traffic {enabled, acknowledged}. Turning it on
// needs acknowledged: true (422 otherwise) and starts the requests afresh,
// also after the ANWB made GDash stop.
func (s *Server) putTraffic(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled      bool `json:"enabled"`
		Acknowledged bool `json:"acknowledged"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	info, err := s.Settings.SetTrafficInfo(r.Context(), body.Enabled, body.Acknowledged)
	if err != nil {
		s.writeSettingsError(w, "set traffic info", err)
		return
	}
	if info.Enabled {
		s.Traffic.Reset()
		s.auditUser(r, auditTrafficEnabled, "")
	} else {
		s.auditUser(r, auditTrafficDisabled, "")
	}
	s.SettingsChanged()
	writeJSON(w, http.StatusOK, info)
}

// statusOf returns the status of one data hub entry, or nil when the hub
// does not have it (nothing on a screen uses it).
func statusOf(entries map[string]datahub.Entry, key string) *sourceStatus {
	entry, ok := entries[key]
	if !ok {
		return nil
	}
	st := entryStatus(entry)
	return &st
}
