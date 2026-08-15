// Package webui embeds chanarr's built React frontend into the Go binary,
// so a single static binary serves the whole app with no separate
// frontend deployment step (spec.md §1).
//
// dist/ is a build artifact — gitignored, not committed — produced by
// `cd web && npm run build`, whose vite.config.ts points build.outDir
// straight at this package's dist/ directory. Build the frontend before
// building chanarr; go:embed requires dist/ to exist and be non-empty.
package webui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded frontend build. The app has no client-side
// routing (a single page, view state held in React) — a path that matches
// no file (a typo, a stale bookmark) 404s rather than falling back to
// index.html, since there's no other valid route for it to be.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("webui: %w", err)
	}
	return http.FileServerFS(sub), nil
}
