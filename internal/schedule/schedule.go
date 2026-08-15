// Package schedule implements chanarr's virtual-timeline domain model.
// See CONTEXT.md and docs/adr/0001-epoch-based-pure-timeline-function.md:
// ProgramAt is a pure function, not a persisted/regenerated schedule, so the
// live streamer and the XMLTV guide writer can never disagree about what's
// airing — both call this same primitive.
package schedule

import "time"

// Channel is a configured source folder plus playback settings, exposed to
// Plex as one tuner lineup entry with its own guide.
type Channel struct {
	ID          int64
	Number      string
	Name        string
	Folder      string
	Shuffle     bool
	ShuffleSeed int64
	// Logo is a filesystem path to the channel's logo image, empty if none
	// is set. Auto-detected from a convention file or uploaded by the user
	// (spec.md §8); internal/httpapi owns writing this field.
	Logo string
}

// Epoch is an immutable, timestamped snapshot of a Channel's playlist —
// its ordered items, their cached durations, and (if shuffled) their
// shuffle order. Any playlist-membership or ordering change stamps a new
// epoch; it is never mutated in place.
type Epoch struct {
	ID        int64
	ChannelID int64
	Start     time.Time
	Items     []PlaylistItem
}

// PlaylistItem is one media file's membership within an Epoch: the file
// plus its cached (ffprobed-once) duration and stream parameters, and its
// position in the epoch's order.
type PlaylistItem struct {
	Path     string
	Duration time.Duration
	Params   StreamParams
}

// StreamParams is the subset of a file's audio/video characteristics
// internal/stream's remux-vs-transcode gate needs (spec.md §7): remux is
// only valid when these match across an epoch's items. Probed once by
// internal/library at scan time and cached here, exactly like Duration —
// internal/stream never re-probes at tune-in, which is what keeps tune-in
// fast regardless of how many episodes a channel has.
type StreamParams struct {
	VideoCodec      string
	Width           int
	Height          int
	FrameRate       float64 // frames per second
	AudioCodec      string
	AudioChannels   int
	AudioSampleRate int
}

// Airing is the result of evaluating a Channel's timeline at a specific
// instant: a PlaylistItem paired with the concrete start/end time it
// occupies for that query. This is the unit both the stream (to pick the
// file and compute a seek offset) and the guide (to render a listing)
// consume.
type Airing struct {
	Item  PlaylistItem
	Start time.Time
	End   time.Time
}

// ProgramAt evaluates what airs on a channel at instant t, within the given
// epoch. The XMLTV guide writer builds a range by calling this repeatedly,
// walking t forward to each Airing's End until its horizon is covered — see
// spec.md §5.
//
// epoch.Items is assumed to already be in final playback order (shuffled at
// epoch-creation time if the channel has shuffle on — see spec.md §2's
// "shuffle fixed per epoch" decision); ProgramAt itself never reorders.
//
// Runtime playback failures (a file missing, ffmpeg erroring) are handled
// by internal/stream substituting filler content that still occupies the
// item's cached Duration — not by ProgramAt — which is what keeps every
// future Airing's boundaries stable regardless of what actually played.
func ProgramAt(epoch Epoch, t time.Time) (Airing, error) {
	if len(epoch.Items) == 0 {
		return Airing{}, ErrEmptyEpoch
	}
	if t.Before(epoch.Start) {
		return Airing{}, ErrBeforeEpochStart
	}

	var cycleLength time.Duration
	for _, item := range epoch.Items {
		cycleLength += item.Duration
	}
	if cycleLength <= 0 {
		return Airing{}, ErrZeroCycleLength
	}

	elapsed := t.Sub(epoch.Start)
	cycleIndex := elapsed / cycleLength
	cycleStart := epoch.Start.Add(cycleIndex * cycleLength)
	pos := elapsed - cycleIndex*cycleLength

	var cum time.Duration
	for _, item := range epoch.Items {
		if pos < cum+item.Duration {
			return Airing{
				Item:  item,
				Start: cycleStart.Add(cum),
				End:   cycleStart.Add(cum + item.Duration),
			}, nil
		}
		cum += item.Duration
	}

	// Unreachable: pos < cycleLength by construction (elapsed - cycleIndex*
	// cycleLength is always in [0, cycleLength)), and the loop above covers
	// exactly [0, cycleLength) across all items.
	return Airing{}, ErrZeroCycleLength
}
