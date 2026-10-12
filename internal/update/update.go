// Package update tells the web interface when a newer GDash release is out.
// Once a day it asks GitHub which release is the latest. Apart from the data
// sources, that is the only request GDash makes: it sends nothing about the
// installation except the running version, in the User-Agent, and it can be
// turned off under Settings or with GDASH_UPDATE_CHECK=off.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultURL is GitHub's latest release of GDash: the newest one that
	// is neither a draft nor a pre-release.
	DefaultURL = "https://api.github.com/repos/stijnvandepol/gdash/releases/latest"
	// releaseNotes is where a release's notes are, followed by its tag.
	releaseNotes = "https://github.com/stijnvandepol/gdash/releases/tag/"
	// checkInterval is how often GDash asks.
	checkInterval = 24 * time.Hour
	// minCheckGap keeps turning the check off and on from asking GitHub
	// every time; unauthenticated callers get 60 requests an hour.
	minCheckGap = time.Hour
	// maxResponse is far above a release's JSON, notes included.
	maxResponse = 1 << 20
)

// Release is a release newer than the running one.
type Release struct {
	// Version is the release's version without a leading v, such as 0.8.0.
	Version string `json:"version"`
	// URL is the page with the release notes.
	URL string `json:"url"`
}

// Checker looks for a newer release once a day while the check is on.
type Checker struct {
	// URL is DefaultURL, or a test server.
	URL       string
	HTTP      *http.Client
	UserAgent string
	// Current is the running version. A development build, whose version
	// is no release number, never checks.
	Current string
	// Enabled reports whether the admin has the check on.
	Enabled func(ctx context.Context) (bool, error)
	// Off is set when GDASH_UPDATE_CHECK=off turns the check off, whatever
	// the setting says.
	Off    bool
	Now    func() time.Time
	Logger *slog.Logger

	wakeOnce sync.Once
	wake     chan struct{}

	mu        sync.Mutex
	enabled   bool
	checkedAt time.Time
	// latest is the newest release GitHub named, newer or not.
	latest *version
	tag    string
}

// Run checks at once and then every day until ctx ends, and again soon
// after Changed. Errors, rate limits included, are only logged at debug
// level: a missing notice is no reason to alarm anyone.
func (c *Checker) Run(ctx context.Context) {
	if c.Off {
		c.Logger.Info("update check turned off by GDASH_UPDATE_CHECK")
		return
	}
	if _, ok := parseVersion(c.Current); !ok {
		c.Logger.Info("no update check for a development build", "version", c.Current)
		return
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-c.wakeChannel():
		}
		if err := c.Check(ctx); err != nil {
			c.Logger.Debug("update check failed; trying again tomorrow", "error", err)
		}
		timer.Reset(checkInterval)
	}
}

// Changed tells a running checker that the setting changed, so turning it
// on shows a newer release without waiting a day. It never blocks.
func (c *Checker) Changed() {
	select {
	case c.wakeChannel() <- struct{}{}:
	default:
	}
}

func (c *Checker) wakeChannel() chan struct{} {
	c.wakeOnce.Do(func() { c.wake = make(chan struct{}, 1) })
	return c.wake
}

// Check asks GitHub for the latest release, unless the check is off or it
// asked within the last hour.
func (c *Checker) Check(ctx context.Context) error {
	if c.Off {
		return nil
	}
	enabled, err := c.Enabled(ctx)
	if err != nil {
		return fmt.Errorf("read the update check setting: %w", err)
	}
	c.mu.Lock()
	c.enabled = enabled
	recent := !c.checkedAt.IsZero() && c.Now().Sub(c.checkedAt) < minCheckGap
	c.mu.Unlock()
	if !enabled || recent {
		return nil
	}
	tag, latest, err := c.fetch(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checkedAt = c.Now()
	if err != nil {
		return err
	}
	c.tag, c.latest = tag, latest
	return nil
}

// Available returns the newer release, or nil when there is none, the
// check is off or it has not answered yet.
func (c *Checker) Available() *Release {
	current, ok := parseVersion(c.Current)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Off || !c.enabled || !ok || c.latest == nil || !c.latest.newerThan(current) {
		return nil
	}
	return &Release{Version: c.latest.String(), URL: releaseNotes + c.tag}
}

// release is the part of GitHub's answer GDash reads.
type release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// fetch returns the tag and version of the latest release; a nil version
// when it is a draft or pre-release, which are not offered.
func (c *Checker) fetch(ctx context.Context) (string, *version, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("ask for the latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 403 and 429 are GitHub's rate limits, 404 a repository without releases.
		return "", nil, fmt.Errorf("latest release: HTTP %d", resp.StatusCode)
	}
	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponse)).Decode(&r); err != nil {
		return "", nil, fmt.Errorf("read the latest release: %w", err)
	}
	v, ok := parseVersion(r.TagName)
	if !ok {
		return "", nil, fmt.Errorf("latest release has the tag %q, which is no version", r.TagName)
	}
	if r.Draft || r.Prerelease || v.pre != "" {
		return "", nil, nil
	}
	return r.TagName, &v, nil
}

// version is a semantic version; build metadata is left out, as it does
// not count in comparisons.
type version struct {
	major, minor, patch int
	pre                 string
}

var semver = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

func parseVersion(s string) (version, bool) {
	m := semver.FindStringSubmatch(s)
	if m == nil {
		return version{}, false
	}
	var v version
	for i, p := range []*int{&v.major, &v.minor, &v.patch} {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return version{}, false
		}
		*p = n
	}
	v.pre = m[4]
	return v, true
}

// newerThan compares two versions. Only a release (without pre-release
// part) is ever compared with the running version, so a pre-release only
// needs to sort below the release with the same numbers.
func (v version) newerThan(o version) bool {
	if v.major != o.major {
		return v.major > o.major
	}
	if v.minor != o.minor {
		return v.minor > o.minor
	}
	if v.patch != o.patch {
		return v.patch > o.patch
	}
	return v.pre == "" && o.pre != ""
}

func (v version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.pre != "" {
		s += "-" + v.pre
	}
	return s
}
