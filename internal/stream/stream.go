// Package stream serves the per-channel MPEG-TS stream chanarr's tuner
// lineup points Plex at. See spec.md §7 and
// .scratch/chanarr-spec/assets/research-stream-pipeline.md; the core
// mechanism (ffmpeg concat + remux-by-default) was proven live in the
// walking-skeleton prototype.
//
// Each streaming cycle — one pass through the channel's playlist starting
// at whatever item schedule.ProgramAt says is airing now — runs as two
// ffmpeg phases:
//
//  1. singleItemCmd plays the remainder of the current item from a plain
//     single-file seek, joining mid-item at tune-in.
//  2. remainderCmd concats whatever comes after it, through the end of the
//     epoch's true order, with its output timestamps shifted to continue
//     where phase 1 left off (-output_ts_offset, computed analytically
//     from the item's cached Duration — no probing needed between phases).
//
// This two-phase split exists because, empirically, this ffmpeg build does
// not reliably honor a seek applied to a concat demuxer input (neither its
// own inpoint directive nor a whole-input -ss meaningfully trims anything
// under -c copy) while a plain single-file -ss does. See playlist.go's doc
// comment on buildRemainderPlaylist for the detail.
//
// When phase 2 reaches its natural end (or either phase fails), the
// handler immediately recomputes ProgramAt for the current time and starts
// a fresh cycle — which lands back at the epoch's first item without
// needing any wraparound logic of its own. A failure serves a
// declared-duration filler clip first, so the failed item's schedule slot
// still elapses correctly (spec.md §2's filler-padding contract) rather
// than the next cycle re-hitting the same failure instantly. Both phases
// are killed on client disconnect via the request context.
//
// A tuner connection observes one fixed epoch snapshot for its whole
// lifetime — the handler never re-fetches mid-stream; a channel edit or
// rescan takes effect on the next tune-in, not retroactively for viewers
// already connected.
package stream

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"chanarr/internal/schedule"
)

// EpochProvider resolves a channel by its tuner GuideNumber and returns its
// current schedule. TODO: implement against internal/store once wired into
// cmd/chanarr/main.go (mirroring internal/tuner's and internal/guide's
// provider pattern).
type EpochProvider func(channelNumber string) (schedule.Channel, schedule.Epoch, error)

// Handler serves GET /stream/{number}.
func Handler(provider EpochProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		number := r.PathValue("number")
		ch, epoch, err := provider(number)
		if err != nil {
			http.Error(w, "channel not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "video/mp2t")
		flusher, _ := w.(http.Flusher)
		ctx := r.Context()

		for ctx.Err() == nil {
			airing, err := schedule.ProgramAt(epoch, time.Now())
			if err != nil {
				// Only reachable on the very first iteration — a
				// structural property of this epoch (e.g. no items),
				// not a transient failure a respawn would fix.
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}

			runCycle(ctx, w, ch.Number, epoch.Items, airing)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

// runCycle plays the remainder of airing's item, then whatever follows it
// through the end of the epoch's true order. If either phase fails, it
// serves filler for whatever remains of airing's declared duration before
// returning, so the caller's next ProgramAt call correctly lands on the
// following item rather than re-hitting the same failure instantly.
func runCycle(ctx context.Context, w io.Writer, channelNumber string, items []schedule.PlaylistItem, airing schedule.Airing) {
	startIndex := indexOfPath(items, airing.Item.Path)
	seek := time.Since(airing.Start)
	if seek < 0 {
		seek = 0
	}
	remux := remuxCompatible(items)
	target := transcodeTarget(items)

	cmd1 := singleItemCmd(ctx, airing.Item.Path, seek, remux, target)
	if err := runToWriter(cmd1, w); err != nil && ctx.Err() == nil {
		log.Printf("stream: channel %s: phase 1 (%s) exited: %v", channelNumber, airing.Item.Path, err)
		serveFiller(ctx, w, channelNumber, time.Until(airing.End))
		return
	}
	if ctx.Err() != nil {
		return
	}

	playlist := buildRemainderPlaylist(items, startIndex)
	if playlist == "" {
		return // startIndex was the epoch's last item — nothing more this cycle.
	}
	playlistPath, cleanup, err := writeTempPlaylist(playlist)
	if err != nil {
		log.Printf("stream: channel %s: build remainder playlist: %v", channelNumber, err)
		serveFiller(ctx, w, channelNumber, time.Until(airing.End))
		return
	}
	defer cleanup()

	tsOffset := airing.Item.Duration - seek
	cmd2 := remainderCmd(ctx, playlistPath, tsOffset, remux, target)
	if err := runToWriter(cmd2, w); err != nil && ctx.Err() == nil {
		log.Printf("stream: channel %s: phase 2 exited: %v", channelNumber, err)
		serveFiller(ctx, w, channelNumber, time.Until(airing.End))
	}
}

func serveFiller(ctx context.Context, w io.Writer, channelNumber string, remaining time.Duration) {
	if remaining <= 0 || ctx.Err() != nil {
		return
	}
	if err := runToWriter(fillerCmd(ctx, remaining), w); err != nil && ctx.Err() == nil {
		log.Printf("stream: channel %s: filler failed: %v", channelNumber, err)
	}
}

func writeTempPlaylist(content string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "chanarr-playlist-*.txt")
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
