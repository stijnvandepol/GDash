package main

import "runtime/debug"

// version is set for release builds with
// -ldflags "-X main.version=v1.2.3".
var version = ""

// shortRevision is long enough to find a commit, short enough to read.
const shortRevision = 12

// currentVersion returns the release version. A development build reports
// "dev" plus the commit Go recorded, if any, so a bug report still says
// what was running.
func currentVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return devVersion(info.Settings)
}

func devVersion(settings []debug.BuildSetting) string {
	var revision string
	var modified bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return "dev"
	}
	if len(revision) > shortRevision {
		revision = revision[:shortRevision]
	}
	if modified {
		revision += "-modified"
	}
	return "dev-" + revision
}
