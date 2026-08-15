// Package stream serves the per-channel MPEG-TS stream chanarr's tuner
// lineup points Plex at. See spec.md §7 and
// .scratch/chanarr-spec/assets/research-stream-pipeline.md; the core
// mechanism (one ffmpeg per item + concat continuity, remux-by-default) was
// proven live in the walking-skeleton prototype.
//
// One ffmpeg process is spawned per tuner request and killed on client
// disconnect; a filler/error-screen generator keeps the TS alive across
// item failures (internal/schedule's filler-padding contract).
package stream

import "net/http"

// Handler serves GET /stream/{channelNumber}. TODO: implement — resolve the
// current Airing via internal/schedule.ProgramAt, compute the tune-in seek
// offset, spawn ffmpeg (remux if the item matches the channel's declared
// parameters, else transcode), and pipe stdout to the response.
func Handler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
