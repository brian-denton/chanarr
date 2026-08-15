// Package library scans a channel's folder and derives filename-based
// metadata. See spec.md §3 and §8.
//
// Rescan: periodic timer (default every 5 minutes) plus a manual "rescan
// now" UI action — no filesystem-watcher dependency in v1 (inotify/FSEvents
// are unreliable under Docker bind mounts). A detected membership/ordering
// change stamps a new internal/schedule.Epoch.
//
// Metadata: SxxExx parsed from filenames via regex for episode-num; falls
// back to the filename (stripped of extension) when unmatched. No online
// lookup (TVDB/TMDB) in v1 — deferred, see spec.md §13. Ordering for
// non-shuffle channels is lexical path/filename sort. Channel logo:
// auto-detect poster.jpg/folder.jpg in the folder, with manual upload as an
// override (internal/httpapi).
package library

import "chanarr/internal/schedule"

// Scan walks a channel's folder and returns a fresh set of playlist items
// (path + ffprobed duration, cached — never re-probed after this). TODO:
// implement folder walk + ffprobe invocation.
func Scan(folder string) ([]schedule.PlaylistItem, error) {
	return nil, errNotImplemented
}

// ParseEpisode extracts season/episode numbers from a filename using the
// SxxExx convention. TODO: implement; ok=false signals the filename-fallback
// path (spec.md §8).
func ParseEpisode(filename string) (season, episode int, ok bool) {
	return 0, 0, false
}
