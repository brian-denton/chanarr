package stream

import (
	"fmt"
	"strings"

	"chanarr/internal/schedule"
)

// buildRemainderPlaylist renders the ffconcat playlist for everything
// after the currently-airing item, through the end of the epoch's true
// order — never wrapping back around to startIndex. The current item's
// own remainder is played separately, by a single-file seek (see
// stream.go's two-phase design and its doc comment for why: this ffmpeg
// build does not reliably honor a seek/inpoint applied to a concat
// demuxer input, confirmed empirically, while a plain single-file -ss
// does). Once this playlist's process reaches its natural end, the
// caller's outer loop recomputes schedule.ProgramAt for the current time
// and starts a fresh cycle — which lands back at the epoch's first item
// (or wherever real time has actually landed) without this needing to
// wrap around itself.
//
// Returns "" if startIndex is the epoch's last item (nothing follows it).
func buildRemainderPlaylist(items []schedule.PlaylistItem, startIndex int) string {
	if startIndex >= len(items)-1 {
		return ""
	}
	var b strings.Builder
	b.WriteString("ffconcat version 1.0\n")
	for _, item := range items[startIndex+1:] {
		fmt.Fprintf(&b, "file %s\n", quoteConcatPath(item.Path))
	}
	return b.String()
}

// quoteConcatPath single-quotes a path for the ffconcat file format,
// escaping any embedded single quotes.
func quoteConcatPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// indexOfPath finds item's position within items by path (paths are unique
// within an epoch). Falls back to 0 if not found — unreachable in practice
// since schedule.ProgramAt always returns an Item drawn from epoch.Items.
func indexOfPath(items []schedule.PlaylistItem, path string) int {
	for i, it := range items {
		if it.Path == path {
			return i
		}
	}
	return 0
}

// remuxCompatible reports whether every item in items shares the first
// item's stream parameters — the spec.md §7 gate, applied at whole-channel
// granularity: a single ffmpeg -c copy process can't selectively transcode
// individual segments within one concat pass, so chanarr's v1 policy is
// all-or-nothing per streaming cycle. A channel whose files are internally
// consistent (the common case for an organized library) remuxes for free;
// a mixed-format channel transcodes uniformly rather than only the
// mismatched items.
func remuxCompatible(items []schedule.PlaylistItem) bool {
	if len(items) == 0 {
		return true
	}
	ref := items[0].Params
	for _, it := range items[1:] {
		if it.Params != ref {
			return false
		}
	}
	return true
}

// transcodeTarget picks what every item gets normalized to when items
// aren't remux-compatible: the first (reference) item's own parameters,
// with safe defaults filled in for anything unset (e.g. an epoch saved
// before StreamParams existed). Encoding a single continuous output across
// differently-sized/rated inputs requires normalizing to one target — this
// is that target.
func transcodeTarget(items []schedule.PlaylistItem) schedule.StreamParams {
	t := items[0].Params
	if t.Width == 0 || t.Height == 0 {
		t.Width, t.Height = 1280, 720
	}
	if t.FrameRate <= 0 {
		t.FrameRate = 30
	}
	if t.AudioSampleRate == 0 {
		t.AudioSampleRate = 48000
	}
	if t.AudioChannels == 0 {
		t.AudioChannels = 2
	}
	return t
}
