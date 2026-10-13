package camera

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/photos"
)

const (
	// MaxBytes is far above a snapshot of a 4K camera, and keeps the
	// latest image of every camera in memory affordable.
	MaxBytes = 5 << 20
	// Timeout bounds one snapshot request; a camera on the LAN answers in
	// well under a second.
	Timeout = 10 * time.Second
	subject = "the camera"
)

// Fetcher fetches snapshots.
type Fetcher struct {
	// HTTP must refuse the addresses that internal/outbound refuses and
	// have a timeout; its redirect policy is replaced by
	// custom.SameHostRedirects, so a secret never goes to another host.
	HTTP      *http.Client
	UserAgent string
}

// Fetch requests cam's snapshot once and returns the image and its content
// type, judged by the content: JPEG, PNG or WebP of at most MaxBytes. Its
// errors are *custom.FetchError, safe to show an admin.
func (f *Fetcher) Fetch(ctx context.Context, cam Camera) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cam.URL, nil)
	if err != nil {
		return nil, "", &custom.FetchError{Message: "the address of this camera is invalid"}
	}
	req.Header.Set("Accept", "image/jpeg, image/png, image/webp")
	if f.UserAgent != "" {
		req.Header.Set("User-Agent", f.UserAgent)
	}
	cam.Auth.Apply(req)
	client := *f.HTTP
	client.CheckRedirect = custom.SameHostRedirects
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", custom.RequestError(subject, req.URL.Host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", custom.StatusError(subject, resp)
	}
	// A video stream never ends; the limit and the client's timeout stop it.
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, "", custom.RequestError(subject, req.URL.Host, err)
	}
	if len(data) > MaxBytes {
		return nil, "", &custom.FetchError{Message: "the camera sent more than 5 MB; use the address of a single snapshot, not a video stream"}
	}
	contentType, ok := photos.ImageType(data)
	if !ok {
		return nil, "", &custom.FetchError{Message: "the camera did not answer with a JPEG, PNG or WebP image; use the address of a snapshot"}
	}
	return data, contentType, nil
}
