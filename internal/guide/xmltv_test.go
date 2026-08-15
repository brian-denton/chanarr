package guide

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"chanarr/internal/schedule"
)

func item(path string, d time.Duration) schedule.PlaylistItem {
	return schedule.PlaylistItem{Path: path, Duration: d}
}

func TestBuildXMLTV_ChannelEntry(t *testing.T) {
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "3", Name: "Late Night Movies"},
		Epoch: schedule.Epoch{
			Start: now,
			Items: []schedule.PlaylistItem{item("/media/movies/feature.mkv", 13*time.Hour)},
		},
	}}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc xmltvDoc
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not well-formed XML: %v\n%s", err, out)
	}

	if len(doc.Channels) != 1 {
		t.Fatalf("got %d channels, want 1", len(doc.Channels))
	}
	ch := doc.Channels[0]
	if ch.ID != "3" {
		t.Errorf("channel id = %q, want %q", ch.ID, "3")
	}
	wantNames := []string{"3 Late Night Movies", "3", "Late Night Movies"}
	if len(ch.DisplayNames) != len(wantNames) {
		t.Fatalf("got %d display-names, want %d: %v", len(ch.DisplayNames), len(wantNames), ch.DisplayNames)
	}
	for i, want := range wantNames {
		if ch.DisplayNames[i] != want {
			t.Errorf("display-name[%d] = %q, want %q", i, ch.DisplayNames[i], want)
		}
	}
}

func TestBuildXMLTV_ProgrammeFields(t *testing.T) {
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "1", Name: "Classic Sitcoms"},
		Epoch: schedule.Epoch{
			Start: now,
			Items: []schedule.PlaylistItem{item("/media/tv/the-one-with-the-pilot.mkv", 24*time.Hour)},
		},
	}}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var doc xmltvDoc
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output is not well-formed XML: %v", err)
	}
	if len(doc.Programmes) != 1 {
		t.Fatalf("got %d programmes, want 1", len(doc.Programmes))
	}
	p := doc.Programmes[0]

	if p.Channel != "1" {
		t.Errorf("programme channel = %q, want %q", p.Channel, "1")
	}
	if p.Title.Value != "Classic Sitcoms" {
		t.Errorf("title = %q, want channel name", p.Title.Value)
	}
	if p.SubTitle == nil || p.SubTitle.Value != "the one with the pilot" {
		t.Errorf("sub-title = %v, want cleaned filename title", p.SubTitle)
	}
	if p.Desc.Value != "Classic Sitcoms — the one with the pilot" {
		t.Errorf("desc = %q, want channel name — episode title", p.Desc.Value)
	}
	if p.Category.Value != "Series" {
		t.Errorf("category = %q, want %q", p.Category.Value, "Series")
	}
	if p.PreviouslyShown == nil {
		t.Error("previously-shown must always be present")
	}
	if len(p.EpisodeNums) != 0 {
		t.Errorf("got episode-num %v, want none (filename has no SxxExx and library.ParseEpisode is unimplemented)", p.EpisodeNums)
	}

	wantStart := "20260815140000 +0000"
	if p.Start != wantStart {
		t.Errorf("start = %q, want %q", p.Start, wantStart)
	}
	wantStop := "20260816140000 +0000"
	if p.Stop != wantStop {
		t.Errorf("stop = %q, want %q", p.Stop, wantStop)
	}
}

func TestBuildXMLTV_NoRatingEmitted(t *testing.T) {
	// chanarr has no rating data; an absent <rating> is honest, a
	// fabricated one isn't (spec.md §5).
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "1", Name: "X"},
		Epoch:   schedule.Epoch{Start: now, Items: []schedule.PlaylistItem{item("/a.mkv", time.Hour)}},
	}}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(out), "<rating") {
		t.Errorf("output must never contain <rating>, got:\n%s", out)
	}
}

func TestBuildXMLTV_CoversHorizonWithoutGapsOrOverlaps(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "1", Name: "X"},
		Epoch: schedule.Epoch{
			Start: now,
			Items: []schedule.PlaylistItem{
				item("/a.mkv", 5*time.Hour),
				item("/b.mkv", 3*time.Hour),
			}, // 8h cycle
		},
	}}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc xmltvDoc
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("not well-formed: %v", err)
	}

	// 12h horizon over an 8h cycle: item A [0,5), B [5,8), A [8,13) — the
	// last programme's own 5h duration carries it past the horizon, which
	// is expected; only its *start* needs to be within [now, horizon).
	if len(doc.Programmes) != 3 {
		t.Fatalf("got %d programmes, want 3: %+v", len(doc.Programmes), doc.Programmes)
	}
	// Adjacent programmes must be contiguous: next Start == prev Stop.
	for i := 1; i < len(doc.Programmes); i++ {
		if doc.Programmes[i].Start != doc.Programmes[i-1].Stop {
			t.Errorf("gap/overlap between programme %d (stop=%s) and %d (start=%s)",
				i-1, doc.Programmes[i-1].Stop, i, doc.Programmes[i].Start)
		}
	}
	// Must cover at least up to the horizon.
	last := doc.Programmes[len(doc.Programmes)-1]
	wantHorizon := now.Add(HorizonHours * time.Hour).Format(timeLayout)
	if last.Stop < wantHorizon {
		t.Errorf("last programme stop %q doesn't reach horizon %q", last.Stop, wantHorizon)
	}
}

func TestBuildXMLTV_MultipleChannels(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{
		{
			Channel: schedule.Channel{Number: "1", Name: "A"},
			Epoch:   schedule.Epoch{Start: now, Items: []schedule.PlaylistItem{item("/a.mkv", time.Hour)}},
		},
		{
			Channel: schedule.Channel{Number: "2", Name: "B"},
			Epoch:   schedule.Epoch{Start: now, Items: []schedule.PlaylistItem{item("/b.mkv", time.Hour)}},
		},
	}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc xmltvDoc
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("not well-formed: %v", err)
	}
	if len(doc.Channels) != 2 {
		t.Fatalf("got %d channels, want 2", len(doc.Channels))
	}
	if len(doc.Programmes) != 24 { // 12h horizon / 1h items, x2 channels
		t.Fatalf("got %d programmes, want 24", len(doc.Programmes))
	}
}

func TestBuildXMLTV_PropagatesScheduleErrors(t *testing.T) {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "1", Name: "Empty"},
		Epoch:   schedule.Epoch{Start: now}, // no items -> schedule.ErrEmptyEpoch
	}}

	_, err := BuildXMLTV(channels, now)
	if err == nil {
		t.Fatal("expected an error for an empty epoch, got nil")
	}
}

func TestBuildXMLTV_EpisodeTitleAndDescFromSonarrStyleFilename(t *testing.T) {
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "1", Name: "The Office"},
		Epoch: schedule.Epoch{
			Start: now,
			Items: []schedule.PlaylistItem{
				item("/tv/The Office - S01E02 - Diversity Day [1080p WEBDL].mkv", 24*time.Hour),
			},
		},
	}}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc xmltvDoc
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	p := doc.Programmes[0]
	if p.SubTitle == nil || p.SubTitle.Value != "Diversity Day" {
		t.Errorf("sub-title = %v, want the episode title cleaned out of the filename", p.SubTitle)
	}
	if p.Desc.Value != "The Office — S01E02 — Diversity Day" {
		t.Errorf("desc = %q", p.Desc.Value)
	}
	if len(p.EpisodeNums) != 2 {
		t.Fatalf("episode-nums = %v, want onscreen + xmltv_ns", p.EpisodeNums)
	}
}

func TestBuildXMLTV_BareSxxExxFilenameOmitsSubTitle(t *testing.T) {
	// "S01E02.mkv" yields no readable title — the sub-title is omitted
	// (the episode-num already tells the story) rather than fabricated.
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	channels := []ChannelSchedule{{
		Channel: schedule.Channel{Number: "1", Name: "The Office"},
		Epoch: schedule.Epoch{
			Start: now,
			Items: []schedule.PlaylistItem{item("/tv/S01E02.mkv", 24*time.Hour)},
		},
	}}

	out, err := BuildXMLTV(channels, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc xmltvDoc
	if err := xml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	p := doc.Programmes[0]
	if p.SubTitle != nil {
		t.Errorf("sub-title = %v, want omitted", p.SubTitle)
	}
	if p.Desc.Value != "The Office — S01E02" {
		t.Errorf("desc = %q", p.Desc.Value)
	}
}
