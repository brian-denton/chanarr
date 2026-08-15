// Package plexlink implements the optional, prompted connection to a Plex
// server that lets chanarr push guide-reload notifications instead of
// waiting on Plex's own ~daily XMLTV re-poll. See spec.md §6 and
// docs/adr/0002-optional-prompted-plex-connection.md.
//
// Never required at onboarding: a channel works and plays without this.
// Uses Plex's PIN-based link flow (plex.tv/link) — the user is shown a
// short code and enters it at plex.tv; chanarr polls for the resulting
// token. No manually-copied X-Plex-Token.
//
// This package is a stateless plex.tv API client — deliberately decoupled
// from persistence, matching internal/guide's approach to the local Plex
// server's DVR key/token. Where the stable clientIdentifier passed to
// StartLink/PollLink gets stored (likely alongside internal/store's
// DeviceID) is the caller's concern, not this package's.
package plexlink

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// apiBase is a var, not a const, so tests can point it at a mock server
// instead of the real plex.tv.
var apiBase = "https://plex.tv"

const productName = "chanarr"

// Pin is a single PIN-link attempt: the code shown to the user and the id
// used to poll for its result.
type Pin struct {
	ID   int64
	Code string
}

// NewClientIdentifier generates a random identifier suitable for the
// clientIdentifier StartLink/PollLink require. Plex ties a PIN to whichever
// identifier created it, so callers must persist and reuse the same value
// across a link attempt (and, ideally, across restarts).
func NewClientIdentifier() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("plexlink: generate client identifier: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func plexHeaders(req *http.Request, clientIdentifier string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Product", productName)
	req.Header.Set("X-Plex-Client-Identifier", clientIdentifier)
}

// StartLink begins a PIN-link attempt via POST /api/v2/pins and returns the
// code to show the user (they enter it at https://plex.tv/link).
func StartLink(ctx context.Context, clientIdentifier string) (Pin, error) {
	// strong=false (the default) is what plex.tv/link expects: a short
	// 4-character code. strong=true instead returns a long-lived code for
	// direct API auth, which the /link page's input can't accept.
	body := strings.NewReader(url.Values{"strong": {"false"}}.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+"/api/v2/pins", body)
	if err != nil {
		return Pin{}, fmt.Errorf("plexlink: build pin request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	plexHeaders(req, clientIdentifier)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Pin{}, fmt.Errorf("plexlink: start link: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return Pin{}, fmt.Errorf("plexlink: start link: unexpected status %d", resp.StatusCode)
	}

	var parsed struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Pin{}, fmt.Errorf("plexlink: decode pin response: %w", err)
	}
	return Pin{ID: parsed.ID, Code: parsed.Code}, nil
}

// PollLink checks GET /api/v2/pins/{id} for whether the user has finished
// linking at plex.tv. linked is false with no error while the pin is still
// awaiting completion — that's the expected, common case, not a failure;
// err is reserved for genuine transport/API problems.
func PollLink(ctx context.Context, clientIdentifier string, pinID int64) (token string, linked bool, err error) {
	pollURL := apiBase + "/api/v2/pins/" + strconv.FormatInt(pinID, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("plexlink: build poll request: %w", err)
	}
	plexHeaders(req, clientIdentifier)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("plexlink: poll link: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("plexlink: poll link: unexpected status %d", resp.StatusCode)
	}

	var parsed struct {
		AuthToken string `json:"authToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", false, fmt.Errorf("plexlink: decode poll response: %w", err)
	}
	return parsed.AuthToken, parsed.AuthToken != "", nil
}
