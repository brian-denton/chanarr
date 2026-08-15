package httpapi

import (
	"net/http"
	"os"
	"strings"
	"time"

	"chanarr/internal/library"
	"chanarr/internal/schedule"
)

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.store.Channels()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	views := make([]channelView, len(channels))
	for i, ch := range channels {
		views[i] = s.toChannelView(ch)
	}
	writeJSON(w, http.StatusOK, views)
}

type createChannelRequest struct {
	Number      string `json:"number"`
	Name        string `json:"name"`
	Folder      string `json:"folder"`
	Shuffle     bool   `json:"shuffle"`
	ShuffleSeed int64  `json:"shuffleSeed,string"` // full int64 range, kept precise across JS's float64 JSON numbers
}

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var req createChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Number == "" || req.Name == "" || req.Folder == "" {
		writeError(w, http.StatusBadRequest, "number, name, and folder are required")
		return
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

	if req.Shuffle && req.ShuffleSeed == 0 {
		seed, err := randomSeed()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		req.ShuffleSeed = seed
	}

	ch := schedule.Channel{
		Number: req.Number, Name: req.Name, Folder: req.Folder,
		Shuffle: req.Shuffle, ShuffleSeed: req.ShuffleSeed,
	}
	ch, err = s.store.SaveChannel(ch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// Logo attachment needs the channel's ID (remote posters are copied to
	// logosDir under it), so it happens after the insert; a failure here
	// just means no auto-logo, never a failed channel creation.
	if name, ok := library.DetectLogo(mount); ok {
		if logoPath, err := s.materializeLogo(mount, name, ch.ID); err == nil {
			ch.Logo = logoPath
			if updated, err := s.store.SaveChannel(ch); err == nil {
				ch = updated
			}
		}
	}

	ordered := applyShuffle(items, ch.Shuffle, ch.ShuffleSeed)
	if _, err := s.store.SaveEpoch(schedule.Epoch{ChannelID: ch.ID, Start: time.Now(), Items: ordered}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, s.toChannelView(ch))
}

type updateChannelRequest struct {
	Number      string `json:"number"`
	Name        string `json:"name"`
	Shuffle     bool   `json:"shuffle"`
	ShuffleSeed int64  `json:"shuffleSeed,string"` // full int64 range, kept precise across JS's float64 JSON numbers
}

// handleUpdateChannel edits name/number/shuffle — not folder, which isn't
// part of the accepted UI's edit panel and would amount to recreating the
// channel rather than editing it.
func (s *Server) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	var req updateChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Number == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "number and name are required")
		return
	}

	existing, err := s.store.Channel(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	// Mirrors handleCreateChannel: turning shuffle on with no seed set gets
	// a fresh random one, so enabling shuffle via an edit doesn't
	// deterministically land every such channel on seed 0.
	if req.Shuffle && req.ShuffleSeed == 0 {
		seed, err := randomSeed()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		req.ShuffleSeed = seed
	}

	// Ticket 08: any shuffle-toggle or seed-change stamps a new epoch,
	// uniformly — plain name/number edits don't touch scheduling at all.
	shuffleChanged := existing.Shuffle != req.Shuffle || existing.ShuffleSeed != req.ShuffleSeed

	existing.Number, existing.Name = req.Number, req.Name
	existing.Shuffle, existing.ShuffleSeed = req.Shuffle, req.ShuffleSeed
	updated, err := s.store.SaveChannel(existing)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	if shuffleChanged {
		mount, err := s.netfs.Mount(updated.Folder)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer mount.Close()

		items, err := library.Scan(mount)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		ordered := applyShuffle(items, updated.Shuffle, updated.ShuffleSeed)
		if _, err := s.store.SaveEpoch(schedule.Epoch{ChannelID: updated.ID, Start: time.Now(), Items: ordered}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, s.toChannelView(updated))
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	// Best-effort cleanup of an uploaded (chanarr-owned) logo file — never
	// touch a path outside logosDir, since an auto-detected logo lives in
	// the user's own media folder.
	if ch, err := s.store.Channel(id); err == nil && ch.Logo != "" && strings.HasPrefix(ch.Logo, s.logosDir) {
		os.Remove(ch.Logo)
	}

	if err := s.store.DeleteChannel(id); err != nil {
		writeStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type rescanResponse struct {
	Changed      bool `json:"changed"`
	EpisodeCount int  `json:"episodeCount"`
}

// handleRescanChannel is the manual "rescan now" action (spec.md §3); the
// periodic 5-minute timer that also calls this same path is wired above
// this package (TODO, not yet implemented — see internal/library's doc
// comment).
func (s *Server) handleRescanChannel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	ch, err := s.store.Channel(id)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	mount, err := s.netfs.Mount(ch.Folder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer mount.Close()

	items, err := library.Scan(mount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var current schedule.Epoch
	if e, err := s.store.CurrentEpoch(id); err == nil {
		current = e
	}

	changed := !itemsEqualAsSet(items, current.Items)
	if changed {
		ordered := applyShuffle(items, ch.Shuffle, ch.ShuffleSeed)
		if _, err := s.store.SaveEpoch(schedule.Epoch{ChannelID: id, Start: time.Now(), Items: ordered}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, rescanResponse{Changed: changed, EpisodeCount: len(items)})
}
