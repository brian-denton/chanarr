// Package library scans a channel's folder and derives filename-based
// metadata. See spec.md §3 and §8.
//
// Rescan: periodic timer (default every 5 minutes) plus a manual "rescan
// now" UI action — no filesystem-watcher dependency in v1 (inotify/FSEvents
// are unreliable under Docker bind mounts). TODO: the timer/manual-trigger
// loop itself lives above this package (it just needs to call Scan and hand
// the result to whatever stamps a new internal/schedule.Epoch on change).
//
// Metadata: SxxExx parsed from filenames via regex for episode-num; falls
// back to the filename (stripped of extension) when unmatched. No online
// lookup (TVDB/TMDB) in v1 — deferred, see spec.md §13. Ordering for
// non-shuffle channels is lexical path/filename sort — Scan always returns
// items in that order; applying a channel's shuffle on top (spec.md §2:
// "shuffle fixed per epoch") is the epoch-creation step's job, not Scan's.
// Channel logo: auto-detect poster.jpg/folder.jpg in the folder, with
// manual upload as an override (internal/httpapi) — TODO, not implemented
// here.
package library

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"chanarr/internal/schedule"
)

// mediaExtensions are the file types Scan treats as playable media,
// matched case-insensitively.
var mediaExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
	".m4v": true, ".ts": true, ".wmv": true, ".flv": true, ".webm": true,
}

func isMediaFile(name string) bool {
	return mediaExtensions[strings.ToLower(filepath.Ext(name))]
}

// Scan walks a channel's folder recursively and returns its playlist items
// — path plus ffprobed duration, cached here and never re-probed after
// this (spec.md §2: durations are fixed at epoch-creation time). Items are
// returned in lexical path order, matching the decided in-order sort rule.
//
// A file that fails to probe (corrupt, mid-copy, unreadable) is logged and
// skipped rather than failing the whole scan — one bad file in a library
// shouldn't take out the rest of the channel. Scan only returns an error
// for a folder-level problem (missing/unreadable folder).
func Scan(folder string) ([]schedule.PlaylistItem, error) {
	var paths []string
	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isMediaFile(d.Name()) {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("library: scan %s: %w", folder, err)
	}
	sort.Strings(paths)

	items := make([]schedule.PlaylistItem, 0, len(paths))
	for _, path := range paths {
		duration, err := probeDuration(path)
		if err != nil {
			log.Printf("library: skipping %s: %v", path, err)
			continue
		}
		items = append(items, schedule.PlaylistItem{Path: path, Duration: duration})
	}
	return items, nil
}

// episodePattern matches the Sonarr/Plex SxxExx naming convention, e.g.
// "S01E02", "s1e2", embedded anywhere in a filename.
var episodePattern = regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,3})`)

// ParseEpisode extracts season/episode numbers from a filename using the
// SxxExx convention. ok=false signals the filename-fallback path (spec.md
// §8) — chanarr never invents episode identity it can't parse.
func ParseEpisode(filename string) (season, episode int, ok bool) {
	m := episodePattern.FindStringSubmatch(filename)
	if m == nil {
		return 0, 0, false
	}
	season, err1 := strconv.Atoi(m[1])
	episode, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return season, episode, true
}
