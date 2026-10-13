package camera

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// idleAfter is how long the image of a camera that no screen asks for is
// kept: twice the longest refresh, so a screen that skips a beat does not
// lose it.
const idleAfter = 10 * time.Minute

// Image is the latest still image of a camera.
type Image struct {
	Data        []byte
	ContentType string
	// TakenAt is when GDash received it.
	TakenAt time.Time
	// Stale is true when the camera failed after this image: it is the
	// last one that arrived, not a current one.
	Stale bool
}

// Keeper holds the latest image of each camera that screens show. Screens
// ask for an image every few seconds; the camera is only asked when its
// last request is at least a refresh interval old, however many screens
// ask. Images live in memory only and are dropped when no screen has asked
// for a while, so nothing of a camera is written to disk.
type Keeper struct {
	Fetcher *Fetcher
	Now     func() time.Time
	Logger  *slog.Logger

	mu      sync.Mutex
	cameras map[int64]*kept
}

type kept struct {
	// mu makes one request to the camera at a time; screens that ask
	// meanwhile wait for its image instead of asking the camera too.
	mu     sync.Mutex
	camera Camera
	image  Image
	tried  time.Time
	err    error
	// used is guarded by Keeper.mu.
	used time.Time
}

// Latest returns cam's newest image, asking the camera first when its last
// request is at least every ago. When the camera fails, the image before
// stays, marked Stale; without any image the camera's error is returned, a
// *custom.FetchError.
func (k *Keeper) Latest(ctx context.Context, cam Camera, every time.Duration) (Image, error) {
	entry := k.entry(cam.ID)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.camera.URL != cam.URL || entry.camera.Auth != cam.Auth {
		// Another address or secret: the old image is not this camera's.
		entry.camera, entry.image, entry.tried, entry.err = cam, Image{}, time.Time{}, nil
	}
	now := k.Now()
	if entry.tried.IsZero() || now.Sub(entry.tried) >= every {
		entry.tried = now
		k.fetch(ctx, entry, now)
	}
	if entry.image.Data == nil {
		return Image{}, entry.err
	}
	image := entry.image
	image.Stale = entry.err != nil
	return image, nil
}

// fetch asks the camera once. The request outlives a screen that goes
// away while it runs: other screens wait for the same image.
func (k *Keeper) fetch(ctx context.Context, entry *kept, now time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), Timeout)
	defer cancel()
	data, contentType, err := k.Fetcher.Fetch(ctx, entry.camera)
	if err != nil {
		if entry.err == nil {
			k.Logger.Warn("camera image could not be fetched; screens keep the last one", "camera", entry.camera.Name, "error", err)
		}
		entry.err = err
		return
	}
	if entry.err != nil {
		k.Logger.Info("camera answers again", "camera", entry.camera.Name)
	}
	entry.image = Image{Data: data, ContentType: contentType, TakenAt: now}
	entry.err = nil
}

// entry returns the camera's entry, and drops those that no screen asked
// for in idleAfter.
func (k *Keeper) entry(id int64) *kept {
	k.mu.Lock()
	defer k.mu.Unlock()
	now := k.Now()
	if k.cameras == nil {
		k.cameras = map[int64]*kept{}
	}
	for other, e := range k.cameras {
		if now.Sub(e.used) > idleAfter {
			delete(k.cameras, other)
		}
	}
	e, ok := k.cameras[id]
	if !ok {
		e = &kept{}
		k.cameras[id] = e
	}
	e.used = now
	return e
}

// Forget drops what is kept of a camera, after it is changed or removed.
func (k *Keeper) Forget(id int64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.cameras, id)
}
