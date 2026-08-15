// Package httpapi is the REST API the embedded React UI (web/) talks to:
// channel CRUD, the onboarding folder-scan step, logo upload, and the
// plexlink PIN-connect flow. See spec.md §9.
package httpapi

import "net/http"

// Mux returns the configured API router. TODO: implement — wire channel
// CRUD (internal/store), folder scan (internal/library), and plexlink
// endpoints.
func Mux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return mux
}
