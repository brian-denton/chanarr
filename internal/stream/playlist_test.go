package stream

import (
	"strings"
	"testing"
	"time"

	"chanarr/internal/schedule"
)

func items(paths ...string) []schedule.PlaylistItem {
	out := make([]schedule.PlaylistItem, len(paths))
	for i, p := range paths {
		out[i] = schedule.PlaylistItem{Path: p, Duration: time.Hour}
	}
	return out
}

func TestBuildRemainderPlaylist_ListsEverythingAfterStartIndex(t *testing.T) {
	its := items("/a.mkv", "/b.mkv", "/c.mkv")
	got := buildRemainderPlaylist(its, 0)

	if strings.Contains(got, "/a.mkv") {
		t.Errorf("must not include the current item itself (played separately by phase 1), got:\n%s", got)
	}
	idxB := strings.Index(got, "/b.mkv")
	idxC := strings.Index(got, "/c.mkv")
	if idxB == -1 || idxC == -1 || idxB > idxC {
		t.Fatalf("expected B then C, got:\n%s", got)
	}
}

func TestBuildRemainderPlaylist_NeverWrapsAround(t *testing.T) {
	its := items("/a.mkv", "/b.mkv", "/c.mkv")
	got := buildRemainderPlaylist(its, 1) // current = B

	if strings.Contains(got, "/a.mkv") || strings.Contains(got, "/b.mkv") {
		t.Errorf("must not wrap back to A or repeat B, got:\n%s", got)
	}
	if !strings.Contains(got, "/c.mkv") {
		t.Errorf("expected C, got:\n%s", got)
	}
}

func TestBuildRemainderPlaylist_EmptyWhenStartIndexIsLast(t *testing.T) {
	its := items("/a.mkv", "/b.mkv")
	got := buildRemainderPlaylist(its, 1) // current = B, the last item
	if got != "" {
		t.Errorf("expected an empty playlist when the current item is last, got:\n%s", got)
	}
}

func TestBuildRemainderPlaylist_EscapesSingleQuotes(t *testing.T) {
	got := buildRemainderPlaylist(items("/a.mkv", "/media/Bob's Show/ep1.mkv"), 0)
	if !strings.Contains(got, `/media/Bob'\''s Show/ep1.mkv`) {
		t.Errorf("expected escaped single quote, got:\n%s", got)
	}
}

func TestIndexOfPath(t *testing.T) {
	its := items("/a.mkv", "/b.mkv", "/c.mkv")
	if got := indexOfPath(its, "/b.mkv"); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := indexOfPath(its, "/nonexistent.mkv"); got != 0 {
		t.Errorf("got %d, want 0 (fallback)", got)
	}
}

func TestRemuxCompatible(t *testing.T) {
	p := schedule.StreamParams{VideoCodec: "h264", Width: 1280, Height: 720, FrameRate: 25, AudioCodec: "aac", AudioChannels: 2, AudioSampleRate: 48000}

	same := []schedule.PlaylistItem{{Path: "/a", Params: p}, {Path: "/b", Params: p}}
	if !remuxCompatible(same) {
		t.Error("expected identical params to be remux-compatible")
	}

	mismatched := []schedule.PlaylistItem{{Path: "/a", Params: p}, {Path: "/b", Params: schedule.StreamParams{VideoCodec: "hevc"}}}
	if remuxCompatible(mismatched) {
		t.Error("expected a codec mismatch to not be remux-compatible")
	}

	if !remuxCompatible(nil) {
		t.Error("expected an empty item list to be trivially remux-compatible")
	}
	if !remuxCompatible([]schedule.PlaylistItem{{Path: "/a", Params: p}}) {
		t.Error("expected a single item to be trivially remux-compatible")
	}
}

func TestTranscodeTarget_FillsDefaultsForUnsetFields(t *testing.T) {
	target := transcodeTarget([]schedule.PlaylistItem{{Path: "/a", Params: schedule.StreamParams{}}})
	if target.Width == 0 || target.Height == 0 || target.FrameRate <= 0 || target.AudioSampleRate == 0 || target.AudioChannels == 0 {
		t.Errorf("expected all zero fields to be defaulted, got %+v", target)
	}
}

func TestTranscodeTarget_UsesFirstItemWhenSet(t *testing.T) {
	p := schedule.StreamParams{Width: 1920, Height: 1080, FrameRate: 24, AudioSampleRate: 44100, AudioChannels: 6}
	target := transcodeTarget([]schedule.PlaylistItem{{Path: "/a", Params: p}, {Path: "/b", Params: schedule.StreamParams{}}})
	if target != p {
		t.Errorf("got %+v, want the first item's params %+v", target, p)
	}
}
