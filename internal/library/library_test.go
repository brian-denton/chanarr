package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chanarr/internal/netfs"
	"chanarr/internal/schedule"
)

// mustMount opens a local netfs mount over dir — Scan and DetectLogo now
// take mounts so folders can also live on SMB/NFS shares.
func mustMount(t *testing.T, dir string) *netfs.Mount {
	t.Helper()
	m, err := netfs.NewManager(nil).Mount(dir)
	if err != nil {
		t.Fatalf("mount %s: %v", dir, err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

func TestParseEpisode(t *testing.T) {
	cases := []struct {
		name        string
		wantSeason  int
		wantEpisode int
		wantOK      bool
	}{
		{"Show.Name.S01E03.Title.mkv", 1, 3, true},
		{"show.name.s1e2.mkv", 1, 2, true},
		{"Show Name - S12E345 - Title.mp4", 12, 345, true},
		{"random-episode-1.mkv", 0, 0, false},
		{"movie-feature.mkv", 0, 0, false},
		{"1x02 - old style.mkv", 0, 0, false}, // v1 only supports SxxExx, not 1x02
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			season, episode, ok := ParseEpisode(c.name)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if ok && (season != c.wantSeason || episode != c.wantEpisode) {
				t.Fatalf("got S%dE%d, want S%dE%d", season, episode, c.wantSeason, c.wantEpisode)
			}
		})
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func withFakeProbe(t *testing.T, fn func(path string) (time.Duration, schedule.StreamParams, error)) {
	t.Helper()
	orig := probeFile
	probeFile = fn
	t.Cleanup(func() { probeFile = orig })
}

func TestScan_RecursiveMediaFilteringAndLexicalOrder(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "ShowA", "Season 1", "S01E02.mkv"))
	mustWriteFile(t, filepath.Join(dir, "ShowA", "Season 1", "S01E01.mkv"))
	mustWriteFile(t, filepath.Join(dir, "ShowA", "Season 2", "S02E01.mkv"))
	mustWriteFile(t, filepath.Join(dir, "poster.jpg")) // non-media, must be skipped
	mustWriteFile(t, filepath.Join(dir, "notes.txt"))  // non-media, must be skipped

	withFakeProbe(t, func(path string) (time.Duration, schedule.StreamParams, error) {
		return 30 * time.Minute, schedule.StreamParams{}, nil
	})

	items, err := Scan(mustMount(t, dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(items), items)
	}
	for _, it := range items {
		if it.Duration != 30*time.Minute {
			t.Errorf("item %s duration = %v, want 30m", it.Path, it.Duration)
		}
	}
	// Lexical order: "Season 1" < "Season 2", "S01E01" < "S01E02".
	if !strings.HasSuffix(items[0].Path, filepath.Join("Season 1", "S01E01.mkv")) {
		t.Errorf("items[0] = %s, want Season 1/S01E01.mkv", items[0].Path)
	}
	if !strings.HasSuffix(items[1].Path, filepath.Join("Season 1", "S01E02.mkv")) {
		t.Errorf("items[1] = %s, want Season 1/S01E02.mkv", items[1].Path)
	}
	if !strings.HasSuffix(items[2].Path, filepath.Join("Season 2", "S02E01.mkv")) {
		t.Errorf("items[2] = %s, want Season 2/S02E01.mkv", items[2].Path)
	}
}

func TestScan_SkipsFilesThatFailToProbe(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "good.mkv"))
	mustWriteFile(t, filepath.Join(dir, "corrupt.mkv"))

	withFakeProbe(t, func(path string) (time.Duration, schedule.StreamParams, error) {
		if strings.Contains(path, "corrupt") {
			return 0, schedule.StreamParams{}, errors.New("ffprobe: invalid data")
		}
		return time.Hour, schedule.StreamParams{}, nil
	})

	items, err := Scan(mustMount(t, dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1 (corrupt.mkv should be skipped, not fail the scan)", len(items))
	}
	if !strings.HasSuffix(items[0].Path, "good.mkv") {
		t.Errorf("got %s, want good.mkv", items[0].Path)
	}
}

func TestDetectLogo_Found(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "Poster.JPG")) // case-insensitive match
	mustWriteFile(t, filepath.Join(dir, "S01E01.mkv"))

	name, ok := DetectLogo(mustMount(t, dir))
	if !ok {
		t.Fatal("expected DetectLogo to find Poster.JPG")
	}
	if name != "Poster.JPG" {
		t.Errorf("got %q, want %q", name, "Poster.JPG")
	}
}

func TestDetectLogo_NotFound(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "S01E01.mkv"))

	_, ok := DetectLogo(mustMount(t, dir))
	if ok {
		t.Fatal("expected DetectLogo to find nothing")
	}
}

func TestDetectLogo_IgnoresSubdirectories(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "Season 1", "poster.jpg")) // not top-level

	_, ok := DetectLogo(mustMount(t, dir))
	if ok {
		t.Fatal("expected DetectLogo to ignore a poster.jpg nested in a subfolder")
	}
}

func TestMount_MissingFolder(t *testing.T) {
	// A bad local path now fails at mount time (netfs stats it up front),
	// before Scan or DetectLogo ever run.
	_, err := netfs.NewManager(nil).Mount("/no/such/folder/chanarr-test")
	if err == nil {
		t.Fatal("expected an error for a missing folder, got nil")
	}
}

func TestScan_EmptyFolder(t *testing.T) {
	dir := t.TempDir()
	items, err := Scan(mustMount(t, dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}

func TestEpisodeTitle(t *testing.T) {
	cases := []struct{ filename, want string }{
		{"The Office - S01E02 - Diversity Day [1080p WEBDL].mkv", "Diversity Day"},
		{"The.Office.S01E02.Diversity.Day.1080p.WEB.x264-GROUP.mkv", "Diversity Day"},
		{"show_s1e2_the_fire_720p.mkv", "the fire"},
		{"S01E02.mkv", ""}, // nothing after the marker
		{"the-one-with-the-pilot.mkv", "the one with the pilot"}, // no SxxExx: whole name cleaned
		{"Movie.Night.2160p.HDR.mkv", "Movie Night"},
	}
	for _, c := range cases {
		if got := EpisodeTitle(c.filename); got != c.want {
			t.Errorf("EpisodeTitle(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}
