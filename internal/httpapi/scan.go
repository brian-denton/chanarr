package httpapi

import (
	"net/http"

	"chanarr/internal/library"
	"chanarr/internal/netfs"
)

type scanRequest struct {
	Folder string `json:"folder"`
	// Optional login for an smb:// folder (NFS has no password auth — its
	// access comes from the server's export rules). Persisted keyed by
	// (protocol, host, share) so later rescans and streaming reuse it;
	// empty means guest, or whatever was stored for this share earlier.
	Username string `json:"username"`
	Password string `json:"password"`
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

	loc, err := netfs.Parse(req.Folder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Save before mounting: the mount itself is what uses these. A typo'd
	// password overwrites a previously-working one, but the retry that
	// fixes the typo overwrites it right back — simpler than staging.
	if loc.Remote() && (req.Username != "" || req.Password != "") {
		if err := s.store.SaveShareCredentials(loc.Protocol, loc.Host, loc.Share, req.Username, req.Password); err != nil {
			writeStoreError(w, err)
			return
		}
	}

	mount, err := s.netfs.Mount(req.Folder)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer mount.Close()

	items, err := library.Scan(mount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := s.store.Channels()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	_, hasLogo := library.DetectLogo(mount)

	writeJSON(w, http.StatusOK, scanResponse{
		Name:         deriveChannelName(req.Folder),
		Number:       nextChannelNumber(existing),
		EpisodeCount: len(items),
		HasLogo:      hasLogo,
	})
}
