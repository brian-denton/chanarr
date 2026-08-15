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
// Channel logo: auto-detect poster.jpg/folder.jpg in the folder via
// DetectLogo, with manual upload as an override (internal/httpapi).
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

	"chanarr/internal/netfs"
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

// Scan walks a channel's mounted folder recursively and returns its
// playlist items — path plus ffprobed duration, cached here and never
// re-probed after this (spec.md §2: durations are fixed at epoch-creation
// time). Items are returned in lexical path order, matching the decided
// in-order sort rule. The mount abstracts where the folder lives (local
// disk, SMB, NFS — internal/netfs); item paths are stored in the mount's
// spec form, and probing reads remote files through the netfs bridge.
//
// A file that fails to probe (corrupt, mid-copy, unreadable) is logged and
// skipped rather than failing the whole scan — one bad file in a library
// shouldn't take out the rest of the channel. Scan only returns an error
// for a folder-level problem (missing/unreadable folder).
func Scan(m *netfs.Mount) ([]schedule.PlaylistItem, error) {
	var rels []string
	err := fs.WalkDir(m.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isMediaFile(d.Name()) {
			return nil
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("library: scan %s: %w", m.Folder(), err)
	}
	sort.Strings(rels)

	items := make([]schedule.PlaylistItem, 0, len(rels))
	for _, rel := range rels {
		spec := m.Spec(rel)
		input, err := m.InputTarget(rel)
		if err != nil {
			log.Printf("library: skipping %s: %v", spec, err)
			continue
		}
		duration, params, err := probeFile(input)
		if err != nil {
			log.Printf("library: skipping %s: %v", spec, err)
			continue
		}
		items = append(items, schedule.PlaylistItem{Path: spec, Duration: duration, Params: params})
	}
	return items, nil
}

// logoCandidates are the conventional filenames Sonarr/Plex-managed
// libraries already tend to have, checked case-insensitively at the
// folder's top level only (spec.md §8 — this is a show-root convention,
// not something to hunt for recursively).
var logoCandidates = []string{
	"poster.jpg", "poster.jpeg", "poster.png",
	"folder.jpg", "folder.jpeg", "folder.png",
}

// DetectLogo looks for a conventional logo/poster file directly in the
// mounted folder's root. It returns the file's name within the folder
// (not a full path — for a remote mount there is no local path; the
// caller copies the bytes out via m.Open if it needs a local file).
// ok=false means none was found — the caller (internal/httpapi) falls
// back to no logo unless the user uploads one.
func DetectLogo(m *netfs.Mount) (name string, ok bool) {
	entries, err := fs.ReadDir(m.FS(), ".")
	if err != nil {
		return "", false
	}
	names := make(map[string]string, len(entries)) // lowercase -> actual
	for _, e := range entries {
		if !e.IsDir() {
			names[strings.ToLower(e.Name())] = e.Name()
		}
	}
	for _, candidate := range logoCandidates {
		if actual, found := names[candidate]; found {
			return actual, true
		}
	}
	return "", false
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
