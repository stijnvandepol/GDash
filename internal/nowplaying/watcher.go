// Package nowplaying follows what plays on the household's Cast devices,
// such as speakers, speaker groups and Nest Hubs, for the Now playing
// widget. It only reads: see cast.Monitor.
package nowplaying

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
)

const (
	// MaxFollowed bounds the devices followed at once, and so the open
	// connections and the size of a snapshot.
	MaxFollowed = 16
	// rediscoverInterval is how often a widget that follows any device
	// looks for devices that were switched on or added.
	rediscoverInterval = 5 * time.Minute
	// discoveryValidFor is how long a device list serves every monitor
	// that looks up its device, so they share one mDNS query.
	discoveryValidFor = time.Minute
	// seekTolerance is how far, in seconds, a reported position may be
	// from where the last one would be by now before screens are told: a
	// seek, not the drift of a clock.
	seekTolerance = 3.0
	// groupModel is the model a speaker group announces.
	groupModel = "Google Cast Group"
)

// States of an Item.
const (
	StatePlaying   = "playing"
	StatePaused    = "paused"
	StateBuffering = "buffering"
)

// Item is what one Cast device plays, as screens receive it.
type Item struct {
	// Name is the device's name, such as "Kitchen speaker".
	Name string `json:"name"`
	// Group is set for a speaker group.
	Group bool   `json:"group,omitempty"`
	App   string `json:"app"`
	Title string `json:"title"`
	// Artist is the artist of a track, the series of an episode or the
	// channel of a video.
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	// Image is the GDash address of the cover art, or empty.
	Image string `json:"image,omitempty"`
	// State is StatePlaying, StatePaused or StateBuffering.
	State string `json:"state"`
	// Position and Duration are in seconds; Duration is 0 for a live
	// stream. Position was read At; while playing it moves on from there.
	Position float64   `json:"position"`
	Duration float64   `json:"duration"`
	At       time.Time `json:"at"`
}

// Want is which devices to follow.
type Want struct {
	// IDs are the devices that widgets name.
	IDs []string
	// Any is set when a widget follows every device that plays.
	Any bool
	// Exclude are devices never to connect to: the TVs that GDash keeps a
	// dashboard on, which the cast keepers own.
	Exclude []string
}

// Watcher follows the devices in its Want, one cast.Monitor each, and
// publishes what they play: device ID to Item, for the devices that play
// or pause something.
type Watcher struct {
	// Discover lists the Cast devices on the network.
	Discover func(ctx context.Context) ([]cast.Device, error)
	// Publish receives the whole value after every change that screens
	// must see. Calls never overlap.
	Publish func(map[string]Item)
	Artwork *Artwork
	Logger  *slog.Logger
	// Follow follows one device until ctx ends, reporting what it plays;
	// nil means a cast.Monitor.
	Follow func(ctx context.Context, resolve func(context.Context) (cast.Device, error), report func(cast.Playing))

	// discoverMu makes monitors that look up their device at the same
	// time share one discovery, which takes seconds.
	discoverMu sync.Mutex
	// knownMu guards the last device list, which targets reads without
	// waiting for a discovery.
	knownMu sync.Mutex
	known   []cast.Device
	knownAt time.Time

	// publishMu keeps calls of Publish in order; published is what the
	// last one handed over.
	publishMu sync.Mutex
	published map[string]Item

	mu      sync.Mutex
	ctx     context.Context
	want    Want
	running map[string]*follower
	items   map[string]Item
	// images are the cover addresses of items, by device ID.
	images   map[string]string
	stopScan context.CancelFunc
}

// follower is one followed device.
type follower struct {
	cancel context.CancelFunc
	device cast.Device
}

// Configure makes want the devices to follow, from now until ctx ends.
func (w *Watcher) Configure(ctx context.Context, want Want) {
	w.mu.Lock()
	w.ctx, w.want = ctx, want
	switch {
	case want.Any && w.stopScan == nil:
		scanCtx, cancel := context.WithCancel(ctx)
		w.stopScan = cancel
		go w.scan(scanCtx)
	case !want.Any && w.stopScan != nil:
		w.stopScan()
		w.stopScan = nil
	}
	w.reconcile()
	w.mu.Unlock()
	w.publish(true)
}

// scan looks for devices now and then, for a widget that follows any.
func (w *Watcher) scan(ctx context.Context) {
	for {
		if _, err := w.devices(ctx, true); err != nil && ctx.Err() == nil {
			w.Logger.Warn("could not look for Cast devices to follow", "error", err)
		}
		w.mu.Lock()
		if ctx.Err() == nil {
			w.reconcile()
		}
		w.mu.Unlock()
		w.publish(false)
		select {
		case <-ctx.Done():
			return
		case <-time.After(rediscoverInterval):
		}
	}
}

// targets returns the devices to follow: those named, and with Any every
// known one, never an excluded one, at most MaxFollowed.
func (w *Watcher) targets() []string {
	ids := slices.Clone(w.want.IDs)
	if w.want.Any {
		w.knownMu.Lock()
		for _, d := range w.known {
			ids = append(ids, d.ID)
		}
		w.knownMu.Unlock()
	}
	ids = slices.DeleteFunc(ids, func(id string) bool { return slices.Contains(w.want.Exclude, id) })
	slices.Sort(ids)
	ids = slices.Compact(ids)
	return ids[:min(len(ids), MaxFollowed)]
}

// reconcile starts and stops monitors to follow the targets. w.mu is held.
func (w *Watcher) reconcile() {
	if w.running == nil {
		w.running, w.items, w.images = map[string]*follower{}, map[string]Item{}, map[string]string{}
	}
	targets := w.targets()
	for id, f := range w.running {
		if !slices.Contains(targets, id) {
			f.cancel()
			delete(w.running, id)
			delete(w.items, id)
			delete(w.images, id)
		}
	}
	for _, id := range targets {
		if _, ok := w.running[id]; ok {
			continue
		}
		ctx, cancel := context.WithCancel(w.ctx)
		f := &follower{cancel: cancel}
		w.running[id] = f
		go w.follow(ctx, id, f)
	}
}

func (w *Watcher) follow(ctx context.Context, id string, f *follower) {
	resolve := func(ctx context.Context) (cast.Device, error) {
		d, err := w.lookup(ctx, id)
		if err == nil {
			w.mu.Lock()
			f.device = d
			w.mu.Unlock()
		}
		return d, err
	}
	report := func(p cast.Playing) { w.report(id, f, p) }
	if w.Follow != nil {
		w.Follow(ctx, resolve, report)
		return
	}
	(&cast.Monitor{Resolve: resolve, Report: report, Logger: w.Logger.With("cast_device", id)}).Run(ctx)
}

// lookup finds a device in a recent device list, or discovers again.
func (w *Watcher) lookup(ctx context.Context, id string) (cast.Device, error) {
	devices, err := w.devices(ctx, false)
	if err != nil {
		return cast.Device{}, err
	}
	for _, d := range devices {
		if d.ID == id {
			return d, nil
		}
	}
	return cast.Device{}, fmt.Errorf("%w: %s; is it switched on?", cast.ErrDeviceNotFound, id)
}

// devices returns the devices on the network: the last list while it is
// recent, unless fresh is set.
func (w *Watcher) devices(ctx context.Context, fresh bool) ([]cast.Device, error) {
	w.discoverMu.Lock()
	defer w.discoverMu.Unlock()
	if !fresh {
		w.knownMu.Lock()
		known, recent := w.known, w.known != nil && time.Since(w.knownAt) < discoveryValidFor
		w.knownMu.Unlock()
		if recent {
			return known, nil
		}
	}
	devices, err := w.Discover(ctx)
	if err != nil {
		return nil, err
	}
	w.knownMu.Lock()
	w.known, w.knownAt = devices, time.Now()
	w.knownMu.Unlock()
	return devices, nil
}

// report takes what a device plays, from its follower f, and publishes it
// when screens must see the change.
func (w *Watcher) report(id string, f *follower, p cast.Playing) {
	w.mu.Lock()
	if w.running[id] != f {
		// The device stopped being followed meanwhile.
		w.mu.Unlock()
		return
	}
	item, playing := w.item(f.device, p)
	previous, had := w.items[id]
	switch {
	case !playing && !had:
		w.mu.Unlock()
		return
	case !playing:
		delete(w.items, id)
		delete(w.images, id)
	case had && unchanged(previous, item):
		w.mu.Unlock()
		return
	default:
		w.items[id] = item
		w.images[id] = p.Image
	}
	w.mu.Unlock()
	w.publish(false)
}

// item turns what a device plays into what screens get; playing is false
// when nothing plays or is paused.
func (w *Watcher) item(device cast.Device, p cast.Playing) (item Item, playing bool) {
	states := map[string]string{cast.PlayerPlaying: StatePlaying, cast.PlayerPaused: StatePaused, cast.PlayerBuffering: StateBuffering}
	state, ok := states[p.State]
	if !ok || (p.Title == "" && p.Artist == "") {
		return Item{}, false
	}
	item = Item{
		Name: device.Name, Group: device.Model == groupModel, App: p.App,
		Title: p.Title, Artist: p.Artist, Album: p.Album, State: state,
		Position: p.Position, Duration: p.Duration, At: p.At,
	}
	if p.Image != "" {
		item.Image = w.Artwork.path(p.Image)
	}
	return item, true
}

// unchanged reports whether next shows the same as previous: the same
// fields, and a position where the previous one has moved on to.
func unchanged(previous, next Item) bool {
	expected := previous.Position
	if previous.State == StatePlaying {
		expected += next.At.Sub(previous.At).Seconds()
	}
	moved := math.Abs(next.Position-expected) > seekTolerance
	previous.Position, previous.At = next.Position, next.At
	return previous == next && !moved
}

// publish hands the current value to Publish, unless it handed over the
// same value last time and force is not set. Configure forces it: a data
// hub entry that was just added waits for its first value.
func (w *Watcher) publish(force bool) {
	w.publishMu.Lock()
	defer w.publishMu.Unlock()
	w.mu.Lock()
	value := maps.Clone(w.items)
	if value == nil {
		value = map[string]Item{}
	}
	images := make([]string, 0, len(w.images))
	for _, address := range w.images {
		if address != "" {
			images = append(images, address)
		}
	}
	w.mu.Unlock()
	if !force && w.published != nil && maps.Equal(value, w.published) {
		return
	}
	w.Artwork.keep(images)
	w.published = value
	w.Publish(value)
}
