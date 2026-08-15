package httpapi

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"chanarr/internal/schedule"
	"chanarr/internal/store"
)

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeStoreError maps the two store-layer conditions httpapi handlers
// actually need to distinguish; anything else is an unexpected failure.
func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "channel not found")
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// channelView is httpapi's channel wire format — deliberately not
// schedule.Channel verbatim: it never exposes the server-side Logo
// filesystem path (the frontend fetches /api/channels/{id}/logo instead),
// and it adds NowPlaying/EpisodeCount computed from the channel's current
// epoch for the dashboard list the UI prototype's winning variant shows.
type channelView struct {
	ID           int64  `json:"id"`
	Number       string `json:"number"`
	Name         string `json:"name"`
	Folder       string `json:"folder"`
	Shuffle      bool   `json:"shuffle"`
	ShuffleSeed  int64  `json:"shuffleSeed"`
	HasLogo      bool   `json:"hasLogo"`
	EpisodeCount int    `json:"episodeCount"`
	NowPlaying   string `json:"nowPlaying,omitempty"`
}

func (s *Server) toChannelView(ch schedule.Channel) channelView {
	v := channelView{
		ID: ch.ID, Number: ch.Number, Name: ch.Name, Folder: ch.Folder,
		Shuffle: ch.Shuffle, ShuffleSeed: ch.ShuffleSeed, HasLogo: ch.Logo != "",
	}
	epoch, err := s.store.CurrentEpoch(ch.ID)
	if err != nil {
		return v // no epoch yet (e.g. an empty folder) — leave NowPlaying blank
	}
	v.EpisodeCount = len(epoch.Items)
	if airing, err := schedule.ProgramAt(epoch, time.Now()); err == nil {
		v.NowPlaying = episodeTitle(airing.Item.Path)
	}
	return v
}

// episodeTitle mirrors internal/guide's sub-title rule: the filename,
// stripped of its extension (spec.md §8's fallback, applied uniformly).
func episodeTitle(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// deriveChannelName turns a folder path into a suggested channel name,
// e.g. "/media/tv/the-office" -> "The Office".
func deriveChannelName(folder string) string {
	base := filepath.Base(filepath.Clean(folder))
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	words := strings.Fields(base)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// nextChannelNumber returns the lowest unused positive integer (as a
// string) among existing channels' numbers, starting from "1".
func nextChannelNumber(existing []schedule.Channel) string {
	used := make(map[int]bool, len(existing))
	for _, c := range existing {
		if n, err := strconv.Atoi(c.Number); err == nil {
			used[n] = true
		}
	}
	for n := 1; ; n++ {
		if !used[n] {
			return strconv.Itoa(n)
		}
	}
}

// randomSeed generates a shuffle seed for a channel that doesn't specify
// one — not security-sensitive, just needs to vary per channel.
func randomSeed() (int64, error) {
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:])), nil
}

// applyShuffle orders items for a new epoch: pass-through in library.Scan's
// lexical order for in-order channels, or a seed-deterministic permutation
// for shuffle ones (ticket 08: "one fixed shuffled order per epoch").
func applyShuffle(items []schedule.PlaylistItem, shuffle bool, seed int64) []schedule.PlaylistItem {
	if !shuffle {
		return items
	}
	ordered := make([]schedule.PlaylistItem, len(items))
	copy(ordered, items)
	rnd := rand.New(rand.NewPCG(uint64(seed), 0))
	rnd.Shuffle(len(ordered), func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	return ordered
}

// itemsEqualAsSet compares two item lists order-independently, by
// (path, duration) pairs — used by the rescan endpoint to decide whether
// anything actually changed (and therefore whether a new epoch is
// warranted at all).
func itemsEqualAsSet(a, b []schedule.PlaylistItem) bool {
	if len(a) != len(b) {
		return false
	}
	durations := make(map[string]time.Duration, len(a))
	for _, it := range a {
		durations[it.Path] = it.Duration
	}
	for _, it := range b {
		d, ok := durations[it.Path]
		if !ok || d != it.Duration {
			return false
		}
	}
	return true
}
