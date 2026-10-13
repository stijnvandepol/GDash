package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// keyTraffic holds whether ANWB traffic info is on, and when the admin
// acknowledged that it is for their own household only.
const keyTraffic = "traffic_info"

// TrafficInfo is the switch for the unofficial ANWB traffic info. It is off
// until an admin turns it on and acknowledges that the data belongs to the
// ANWB and is for their own household only.
type TrafficInfo struct {
	Enabled bool `json:"enabled"`
	// AcknowledgedAt is when the admin last acknowledged, if ever.
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

// TrafficInfo returns the switch; it is off when it was never set.
func (s *Service) TrafficInfo(ctx context.Context) (TrafficInfo, error) {
	value, ok, err := s.Store.Setting(ctx, keyTraffic)
	if err != nil || !ok {
		return TrafficInfo{}, err
	}
	var info TrafficInfo
	if err := json.Unmarshal([]byte(value), &info); err != nil {
		return TrafficInfo{}, fmt.Errorf("stored traffic info setting is invalid: %w", err)
	}
	// On without an acknowledgement is not on.
	info.Enabled = info.Enabled && info.AcknowledgedAt != nil
	return info, nil
}

// SetTrafficInfo turns ANWB traffic info on or off. Turning it on needs the
// acknowledgement every time; without it this returns a *validation.Error
// and nothing changes. Turning it off keeps the time of the last
// acknowledgement.
func (s *Service) SetTrafficInfo(ctx context.Context, enabled, acknowledged bool) (TrafficInfo, error) {
	info, err := s.TrafficInfo(ctx)
	if err != nil {
		return TrafficInfo{}, err
	}
	if enabled && !acknowledged {
		var errs validation.Collector
		errs.Add("acknowledged", "confirm that you use this for your own household only and that the data belongs to the ANWB")
		return TrafficInfo{}, errs.Err()
	}
	info.Enabled = enabled
	if enabled {
		now := s.Now().UTC()
		info.AcknowledgedAt = &now
	}
	value, err := json.Marshal(info)
	if err != nil {
		return TrafficInfo{}, err
	}
	if err := s.Store.SetSetting(ctx, keyTraffic, string(value)); err != nil {
		return TrafficInfo{}, err
	}
	return info, nil
}
