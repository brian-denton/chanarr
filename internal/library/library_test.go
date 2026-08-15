package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func withFakeProbe(t *testing.T, fn func(path string) (time.Duration, error)) {
	t.Helper()
	orig := probeDuration
	probeDuration = fn
	t.Cleanup(func() { probeDuration = orig })
}

func TestScan_RecursiveMediaFilteringAndLexicalOrder(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "ShowA", "Season 1", "S01E02.mkv"))
	mustWriteFile(t, filepath.Join(dir, "ShowA", "Season 1", "S01E01.mkv"))
	mustWriteFile(t, filepath.Join(dir, "ShowA", "Season 2", "S02E01.mkv"))
	mustWriteFile(t, filepath.Join(dir, "poster.jpg")) // non-media, must be skipped
	mustWriteFile(t, filepath.Join(dir, "notes.txt"))  // non-media, must be skipped

	withFakeProbe(t, func(path string) (time.Duration, error) { return 30 * time.Minute, nil })

	items, err := Scan(dir)
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

	withFakeProbe(t, func(path string) (time.Duration, error) {
		if strings.Contains(path, "corrupt") {
			return 0, errors.New("ffprobe: invalid data")
		}
		return time.Hour, nil
	})

	items, err := Scan(dir)
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

func TestScan_MissingFolder(t *testing.T) {
	_, err := Scan("/no/such/folder/chanarr-test")
	if err == nil {
		t.Fatal("expected an error for a missing folder, got nil")
	}
}

func TestScan_EmptyFolder(t *testing.T) {
	dir := t.TempDir()
	items, err := Scan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("got %d items, want 0", len(items))
	}
}
