package httpapi

import (
	"net/http"
	"strconv"
)

// linkAttempt is process-local, in-memory state for a PIN-link attempt
// currently in flight — never persisted. The account token only ever lives
// here transiently (between poll succeeding and the user confirming their
// local Plex server's address) and is never sent back to the frontend
// (docs/adr/0002; keeping an account-level auth token out of the browser
// is a deliberate security choice, not an oversight).
type linkAttempt struct {
	clientIdentifier string
	linked           bool
	token            string
}

type startLinkResponse struct {
	PinID int64  `json:"pinId"`
	Code  string `json:"code"`
}

func (s *Server) handleStartLink(w http.ResponseWriter, r *http.Request) {
	clientID, err := s.store.PlexClientIdentifier()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pin, err := s.startLink(r.Context(), clientID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	s.mu.Lock()
	s.linkAttempts[pin.ID] = &linkAttempt{clientIdentifier: clientID}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, startLinkResponse{PinID: pin.ID, Code: pin.Code})
}

type pollLinkResponse struct {
	Linked bool `json:"linked"`
}

func (s *Server) handlePollLink(w http.ResponseWriter, r *http.Request) {
	pinID, err := strconv.ParseInt(r.URL.Query().Get("pinId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or missing pinId")
		return
	}

	s.mu.Lock()
	attempt, ok := s.linkAttempts[pinID]
	s.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "unknown or expired link attempt")
		return
	}
	if attempt.linked {
		writeJSON(w, http.StatusOK, pollLinkResponse{Linked: true})
		return
	}

	token, linked, err := s.pollLink(r.Context(), attempt.clientIdentifier, pinID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if linked {
		s.mu.Lock()
		attempt.linked, attempt.token = true, token
		s.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, pollLinkResponse{Linked: linked})
}

type saveConnectionRequest struct {
	PinID     int64  `json:"pinId"`
	ServerURL string `json:"serverUrl"`
}

// handleSaveConnection finalizes a completed link attempt into a persisted
// Plex connection once the user supplies their local PMS address — plex.tv
// only authenticates the account, it doesn't tell chanarr which server on
// the LAN to push guide reloads to.
func (s *Server) handleSaveConnection(w http.ResponseWriter, r *http.Request) {
	var req saveConnectionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ServerURL == "" {
		writeError(w, http.StatusBadRequest, "serverUrl is required")
		return
	}

	s.mu.Lock()
	attempt, ok := s.linkAttempts[req.PinID]
	s.mu.Unlock()
	if !ok || !attempt.linked {
		writeError(w, http.StatusConflict, "complete the link at plex.tv/link first")
		return
	}

	if err := s.store.SavePlexConnection(req.ServerURL, attempt.token); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.mu.Lock()
	delete(s.linkAttempts, req.PinID)
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]bool{"connected": true})
}

type connectionResponse struct {
	ServerURL string `json:"serverUrl"`
	Connected bool   `json:"connected"`
}

// handleGetConnection never returns the stored token — the frontend only
// needs to know whether a connection exists.
func (s *Server) handleGetConnection(w http.ResponseWriter, r *http.Request) {
	serverURL, _, connected, err := s.store.PlexConnection()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connectionResponse{ServerURL: serverURL, Connected: connected})
}
