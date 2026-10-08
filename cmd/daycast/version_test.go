package main

import (
	"runtime/debug"
	"testing"
)

func TestDevVersion(t *testing.T) {
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"no vcs info", nil, "dev"},
		{"clean commit", []debug.BuildSetting{{Key: "vcs.revision", Value: "9fbdb93c1d2e3f4a5b6c7d8e9f00112233445566"}, {Key: "vcs.modified", Value: "false"}}, "dev-9fbdb93c1d2e"},
		{"local changes", []debug.BuildSetting{{Key: "vcs.revision", Value: "9fbdb93"}, {Key: "vcs.modified", Value: "true"}}, "dev-9fbdb93-modified"},
	}
	for _, tt := range tests {
		if got := devVersion(tt.settings); got != tt.want {
			t.Errorf("%s: devVersion = %q, want %q", tt.name, got, tt.want)
		}
	}
}
