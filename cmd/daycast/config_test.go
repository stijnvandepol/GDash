package main

import (
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"DAYCAST_LATITUDE": "51.6461", "DAYCAST_LONGITUDE": "5.9449"}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.addr != ":8420" || cfg.dataDir != "data" || cfg.castDevice != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.location.Latitude != 51.65 || cfg.location.Longitude != 5.94 {
		t.Fatalf("location = %+v, want rounded to 2 decimals", cfg.location)
	}
}

func TestLoadConfigCastDevice(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"DAYCAST_LATITUDE": "51.65", "DAYCAST_LONGITUDE": "5.95",
		"DAYCAST_CAST_DEVICE": "192.168.1.40", "DAYCAST_CAST_APP_ID": "ABCD1234",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.castDevice != "192.168.1.40:8009" || cfg.castAppID != "ABCD1234" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"missing location", map[string]string{}, "DAYCAST_LATITUDE"},
		{"latitude not a number", map[string]string{"DAYCAST_LATITUDE": "noord", "DAYCAST_LONGITUDE": "5"}, "not a number"},
		{"latitude out of range", map[string]string{"DAYCAST_LATITUDE": "95", "DAYCAST_LONGITUDE": "5"}, "invalid location"},
		{"device without app id", map[string]string{"DAYCAST_LATITUDE": "51", "DAYCAST_LONGITUDE": "5", "DAYCAST_CAST_DEVICE": "10.0.0.2"}, "both"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(env(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}
