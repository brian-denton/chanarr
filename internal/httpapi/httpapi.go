// Package httpapi is the REST API the embedded React UI (web/) talks to:
// channel CRUD, the onboarding folder-scan step, logo upload, and the
// plexlink PIN-connect flow. See spec.md §9.
package httpapi

import (
	"context"
	"net/http"
	"sync"

	"chanarr/internal/netfs"
	"chanarr/internal/plexlink"
	"chanarr/internal/store"
)

// Server holds httpapi's dependencies and the small amount of in-process
// state a Plex PIN-link attempt needs while it's in flight (never
// persisted — see plexconnect.go).
type Server struct {
	store    store.Store
	logosDir string
	netfs    *netfs.Manager

	// startLink/pollLink default to plexlink's real functions; injected as
	// fields (matching internal/tuner's and internal/guide's provider-func
	// style) so tests can substitute a fake without an unexported-field
	// reach-across into plexlink's package.
	startLink func(ctx context.Context, clientIdentifier string) (plexlink.Pin, error)
	pollLink  func(ctx context.Context, clientIdentifier string, pinID int64) (token string, linked bool, err error)

	mu           sync.Mutex
	linkAttempts map[int64]*linkAttempt
}

// NewServer builds a Server. logosDir is where uploaded channel logos are
// stored (config.Config.LogosDir). nfs mounts channel folders wherever
// they live — local disk or a network share (internal/netfs).
func NewServer(st store.Store, logosDir string, nfs *netfs.Manager) *Server {
	return &Server{
		store:        st,
		logosDir:     logosDir,
		netfs:        nfs,
		startLink:    plexlink.StartLink,
		pollLink:     plexlink.PollLink,
		linkAttempts: make(map[int64]*linkAttempt),
	}
}

// Mux returns the configured API router.
func (s *Server) Mux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("POST /api/scan", s.handleScan)

	mux.HandleFunc("GET /api/channels", s.handleListChannels)
	mux.HandleFunc("POST /api/channels", s.handleCreateChannel)
	mux.HandleFunc("PUT /api/channels/{id}", s.handleUpdateChannel)
	mux.HandleFunc("DELETE /api/channels/{id}", s.handleDeleteChannel)
	mux.HandleFunc("POST /api/channels/{id}/rescan", s.handleRescanChannel)
	mux.HandleFunc("POST /api/channels/{id}/logo", s.handleUploadLogo)
	mux.HandleFunc("GET /api/channels/{id}/logo", s.handleServeLogo)

	mux.HandleFunc("POST /api/plex/link/start", s.handleStartLink)
	mux.HandleFunc("GET /api/plex/link/poll", s.handlePollLink)
	mux.HandleFunc("POST /api/plex/connection", s.handleSaveConnection)
	mux.HandleFunc("GET /api/plex/connection", s.handleGetConnection)

	return mux
}
