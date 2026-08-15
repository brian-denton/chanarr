package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DVR identifies one Live TV & DVR entry on the Plex server, along with its
// associated tuner device key. See research-plex-guide-xmltv.md §3.
type DVR struct {
	Key       string
	DeviceKey string
}

// ListDVRs fetches the Plex server's configured DVRs via GET /livetv/dvrs.
// Resolving which one corresponds to chanarr's own tuner (by DeviceKey) is
// the caller's job — internal/plexlink/internal/store's, once channel and
// connection persistence exist — not this package's.
func ListDVRs(ctx context.Context, serverURL, token string) ([]DVR, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serverURL+"/livetv/dvrs", nil)
	if err != nil {
		return nil, fmt.Errorf("guide: build dvrs request: %w", err)
	}
	req.Header.Set("X-Plex-Token", token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("guide: list dvrs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("guide: list dvrs: unexpected status %d", resp.StatusCode)
	}

	var parsed struct {
		MediaContainer struct {
			Dvr []struct {
				Key    string `json:"key"`
				Device struct {
					Key string `json:"key"`
				} `json:"Device"`
			} `json:"Dvr"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("guide: decode dvrs response: %w", err)
	}

	dvrs := make([]DVR, len(parsed.MediaContainer.Dvr))
	for i, d := range parsed.MediaContainer.Dvr {
		dvrs[i] = DVR{Key: d.Key, DeviceKey: d.Device.Key}
	}
	return dvrs, nil
}

// PushReload triggers Plex's POST /livetv/dvrs/{dvrKey}/reloadGuide after a
// guide regeneration — Plex's own XMLTV re-poll is roughly daily and
// unreliable, so every serious implementation pushes this explicitly
// (research-plex-guide-xmltv.md §3). Callers resolve dvrKey via ListDVRs.
func PushReload(ctx context.Context, serverURL, token, dvrKey string) error {
	url := serverURL + "/livetv/dvrs/" + dvrKey + "/reloadGuide"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("guide: build reloadGuide request: %w", err)
	}
	req.Header.Set("X-Plex-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("guide: push reloadGuide: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("guide: push reloadGuide: unexpected status %d", resp.StatusCode)
	}
	return nil
}
