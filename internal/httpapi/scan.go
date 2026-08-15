package httpapi

import (
	"net/http"

	"chanarr/internal/library"
)

type scanRequest struct {
	Folder string `json:"folder"`
}

type scanResponse struct {
	Name         string `json:"name"`
	Number       string `json:"number"`
	EpisodeCount int    `json:"episodeCount"`
	HasLogo      bool   `json:"hasLogo"`
}

// handleScan is the onboarding step: point at a folder, see what chanarr
// found, before committing to creating a channel from it (spec.md §9's
// wizard flow — this only previews, POST /api/channels does the creating).
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Folder == "" {
		writeError(w, http.StatusBadRequest, "folder is required")
		return
	}

	items, err := library.Scan(req.Folder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.Channels()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	_, hasLogo := library.DetectLogo(req.Folder)

	writeJSON(w, http.StatusOK, scanResponse{
		Name:         deriveChannelName(req.Folder),
		Number:       nextChannelNumber(existing),
		EpisodeCount: len(items),
		HasLogo:      hasLogo,
	})
}
