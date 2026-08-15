package schedule

import (
	"testing"
	"time"
)

func mustEpoch(start time.Time, durations ...time.Duration) Epoch {
	items := make([]PlaylistItem, len(durations))
	for i, d := range durations {
		items[i] = PlaylistItem{Path: string(rune('A' + i)), Duration: d}
	}
	return Epoch{Start: start, Items: items}
}

func TestProgramAt_WithinFirstCycle(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 30*time.Minute, 20*time.Minute)

	// 10 minutes in: still item A (ends at 30m).
	airing, err := ProgramAt(epoch, start.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if airing.Item.Path != "A" || !airing.Start.Equal(start) || !airing.End.Equal(start.Add(30*time.Minute)) {
		t.Fatalf("got %+v", airing)
	}
}

func TestProgramAt_ItemBoundaryIsExclusiveOnEnd(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 30*time.Minute, 20*time.Minute)

	// Exactly at the 30m boundary: should be item B (A's End is exclusive).
	airing, err := ProgramAt(epoch, start.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if airing.Item.Path != "B" {
		t.Fatalf("expected item B at exact boundary, got %q", airing.Item.Path)
	}
}

func TestProgramAt_WrapsAcrossCycles(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 30*time.Minute, 20*time.Minute) // 50m cycle

	// 65 minutes in = 15 minutes into the second cycle -> item A again,
	// but Start/End must reflect the *second* cycle, not the first.
	airing, err := ProgramAt(epoch, start.Add(65*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantStart := start.Add(50 * time.Minute)
	wantEnd := start.Add(80 * time.Minute)
	if airing.Item.Path != "A" || !airing.Start.Equal(wantStart) || !airing.End.Equal(wantEnd) {
		t.Fatalf("got %+v, want start=%v end=%v", airing, wantStart, wantEnd)
	}
}

func TestProgramAt_ManyCyclesAndTrailingItem(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 10*time.Minute, 10*time.Minute, 10*time.Minute) // 30m cycle

	// 7 cycles + 25 minutes = the third item's window [20m,30m) of cycle 8.
	at := start.Add(7*30*time.Minute + 25*time.Minute)
	airing, err := ProgramAt(epoch, at)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if airing.Item.Path != "C" {
		t.Fatalf("expected item C, got %q", airing.Item.Path)
	}
	wantStart := start.Add(7*30*time.Minute + 20*time.Minute)
	if !airing.Start.Equal(wantStart) {
		t.Fatalf("got start %v, want %v", airing.Start, wantStart)
	}
}

func TestProgramAt_ExactlyAtEpochStart(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 30*time.Minute)

	airing, err := ProgramAt(epoch, start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !airing.Start.Equal(start) {
		t.Fatalf("got start %v, want %v", airing.Start, start)
	}
}

func TestProgramAt_BeforeEpochStart(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 30*time.Minute)

	_, err := ProgramAt(epoch, start.Add(-time.Second))
	if err != ErrBeforeEpochStart {
		t.Fatalf("got err %v, want ErrBeforeEpochStart", err)
	}
}

func TestProgramAt_EmptyEpoch(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := Epoch{Start: start}

	_, err := ProgramAt(epoch, start)
	if err != ErrEmptyEpoch {
		t.Fatalf("got err %v, want ErrEmptyEpoch", err)
	}
}

func TestProgramAt_ZeroCycleLength(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 0, 0)

	_, err := ProgramAt(epoch, start)
	if err != ErrZeroCycleLength {
		t.Fatalf("got err %v, want ErrZeroCycleLength", err)
	}
}

// A guide range query is just repeated ProgramAt calls walking to each
// Airing's End — this test exercises that pattern directly, matching how
// internal/guide will use ProgramAt (spec.md §5).
func TestProgramAt_RangeQueryViaRepeatedCalls(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	epoch := mustEpoch(start, 30*time.Minute, 20*time.Minute) // 50m cycle
	horizon := start.Add(2 * time.Hour)

	var gotPaths []string
	for at := start; at.Before(horizon); {
		airing, err := ProgramAt(epoch, at)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		gotPaths = append(gotPaths, airing.Item.Path)
		at = airing.End
	}

	want := []string{"A", "B", "A", "B", "A"} // 2h / 50m cycle = 2 full cycles + partial
	if len(gotPaths) != len(want) {
		t.Fatalf("got %v, want %v", gotPaths, want)
	}
	for i := range want {
		if gotPaths[i] != want[i] {
			t.Fatalf("got %v, want %v", gotPaths, want)
		}
	}
}
