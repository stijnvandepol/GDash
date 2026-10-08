// Package web embeds the built frontend (npm run build in this directory)
// into the Daycast binary, so a release is a single file.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the built frontend with dist/ as its root.
func Dist() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
