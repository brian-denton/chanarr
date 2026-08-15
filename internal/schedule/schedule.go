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
// plus its cached (ffprobed-once) duration and its position in the
// epoch's order.
type PlaylistItem struct {
	Path     string
	Duration time.Duration
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

// ProgramAt evaluates what airs on a channel at instant t, within the
// given epoch. The XMLTV guide writer builds a range by calling this
// repeatedly, walking t forward to each Airing's End until its horizon is
// covered — see spec.md §5. Not yet implemented: cycle-length math over
// epoch.Items, including filler-padding for items whose expected duration
// couldn't be honored at playback time (spec.md §2).
func ProgramAt(epoch Epoch, t time.Time) (Airing, error) {
	// TODO: compute position-in-cycle as (t - epoch.Start) mod cycleLength,
	// walk epoch.Items (in epoch-stamped shuffle order if applicable) to the
	// item covering that offset.
	return Airing{}, errNotImplemented
}
