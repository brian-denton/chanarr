package store

import (
	"errors"
	"testing"
	"time"

	"chanarr/internal/schedule"
)

func openTestStore(t *testing.T) Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestChannels_EmptyInitially(t *testing.T) {
	s := openTestStore(t)
	channels, err := s.Channels()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 0 {
		t.Fatalf("got %d channels, want 0", len(channels))
	}
}

func TestSaveChannel_InsertAssignsID(t *testing.T) {
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "Classic Sitcoms", Folder: "/media/tv/sitcoms"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.ID == 0 {
		t.Fatal("expected a non-zero ID after insert")
	}

	channels, err := s.Channels()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 1 || channels[0].Name != "Classic Sitcoms" {
		t.Fatalf("got %+v, want one channel named Classic Sitcoms", channels)
	}
}

func TestSaveChannel_UpdateExisting(t *testing.T) {
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "Old Name", Folder: "/a"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	ch.Name = "New Name"
	ch.Shuffle = true
	ch.ShuffleSeed = 42
	if _, err := s.SaveChannel(ch); err != nil {
		t.Fatalf("update: %v", err)
	}

	channels, err := s.Channels()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("got %d channels, want 1 (update must not insert a new row)", len(channels))
	}
	got := channels[0]
	if got.Name != "New Name" || !got.Shuffle || got.ShuffleSeed != 42 {
		t.Fatalf("got %+v, want updated fields", got)
	}
}

func TestSaveChannel_UpdateNonexistentReturnsNotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.SaveChannel(schedule.Channel{ID: 999, Number: "1", Name: "X", Folder: "/a"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got err %v, want ErrNotFound", err)
	}
}

func TestDeleteChannel(t *testing.T) {
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "X", Folder: "/a"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := s.DeleteChannel(ch.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	channels, _ := s.Channels()
	if len(channels) != 0 {
		t.Fatalf("got %d channels after delete, want 0", len(channels))
	}
}

func TestDeleteChannel_NonexistentReturnsNotFound(t *testing.T) {
	s := openTestStore(t)
	err := s.DeleteChannel(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got err %v, want ErrNotFound", err)
	}
}

func TestDeleteChannel_CascadesToEpochs(t *testing.T) {
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "X", Folder: "/a"})
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	_, err = s.SaveEpoch(schedule.Epoch{
		ChannelID: ch.ID,
		Start:     time.Now().UTC(),
		Items:     []schedule.PlaylistItem{{Path: "/a/1.mkv", Duration: time.Hour}},
	})
	if err != nil {
		t.Fatalf("insert epoch: %v", err)
	}

	if err := s.DeleteChannel(ch.ID); err != nil {
		t.Fatalf("delete channel: %v", err)
	}
	if _, err := s.CurrentEpoch(ch.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got err %v, want ErrNotFound after cascading delete", err)
	}
}

func TestCurrentEpoch_NoneYetReturnsNotFound(t *testing.T) {
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "X", Folder: "/a"})
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	_, err = s.CurrentEpoch(ch.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got err %v, want ErrNotFound", err)
	}
}

func TestSaveEpoch_RoundTripsItemsInOrder(t *testing.T) {
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "X", Folder: "/a"})
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	start := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	items := []schedule.PlaylistItem{
		{Path: "/a/ep1.mkv", Duration: 30 * time.Minute},
		{Path: "/a/ep2.mkv", Duration: 45 * time.Minute},
		{Path: "/a/ep3.mkv", Duration: time.Hour},
	}
	saved, err := s.SaveEpoch(schedule.Epoch{ChannelID: ch.ID, Start: start, Items: items})
	if err != nil {
		t.Fatalf("SaveEpoch: %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("expected a non-zero epoch ID")
	}

	got, err := s.CurrentEpoch(ch.ID)
	if err != nil {
		t.Fatalf("CurrentEpoch: %v", err)
	}
	if got.ID != saved.ID {
		t.Errorf("got epoch ID %d, want %d", got.ID, saved.ID)
	}
	if !got.Start.Equal(start) {
		t.Errorf("got Start %v, want %v", got.Start, start)
	}
	if len(got.Items) != len(items) {
		t.Fatalf("got %d items, want %d", len(got.Items), len(items))
	}
	for i, want := range items {
		if got.Items[i].Path != want.Path || got.Items[i].Duration != want.Duration {
			t.Errorf("item %d = %+v, want %+v", i, got.Items[i], want)
		}
	}
}

func TestSaveEpoch_NeverUpdatesPriorEpoch(t *testing.T) {
	// docs/adr/0001: epochs are immutable — a new SaveEpoch call always
	// creates a fresh row, never mutates the previous one.
	s := openTestStore(t)
	ch, err := s.SaveChannel(schedule.Channel{Number: "1", Name: "X", Folder: "/a"})
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	first, err := s.SaveEpoch(schedule.Epoch{
		ChannelID: ch.ID,
		Start:     time.Now().UTC().Add(-time.Hour),
		Items:     []schedule.PlaylistItem{{Path: "/a/old.mkv", Duration: time.Hour}},
	})
	if err != nil {
		t.Fatalf("save first epoch: %v", err)
	}
	second, err := s.SaveEpoch(schedule.Epoch{
		ChannelID: ch.ID,
		Start:     time.Now().UTC(),
		Items:     []schedule.PlaylistItem{{Path: "/a/new.mkv", Duration: time.Hour}},
	})
	if err != nil {
		t.Fatalf("save second epoch: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("expected a distinct epoch ID for the second SaveEpoch call")
	}

	current, err := s.CurrentEpoch(ch.ID)
	if err != nil {
		t.Fatalf("CurrentEpoch: %v", err)
	}
	if current.ID != second.ID {
		t.Errorf("CurrentEpoch returned %d, want the most recent epoch %d", current.ID, second.ID)
	}
}

func TestDeviceID_StableAcrossCalls(t *testing.T) {
	s := openTestStore(t)
	id1, err := s.DeviceID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id1 == "" {
		t.Fatal("expected a non-empty DeviceID")
	}
	id2, err := s.DeviceID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("DeviceID changed across calls: %q vs %q", id1, id2)
	}
}

func TestDeviceID_StableAcrossReopen(t *testing.T) {
	dbPath := t.TempDir() + "/chanarr.db"

	s1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	id1, err := s1.DeviceID()
	if err != nil {
		t.Fatalf("DeviceID: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	id2, err := s2.DeviceID()
	if err != nil {
		t.Fatalf("DeviceID after reopen: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("DeviceID not stable across restart: %q vs %q", id1, id2)
	}
}

func TestPlexConnection_DisconnectedInitially(t *testing.T) {
	s := openTestStore(t)
	serverURL, token, connected, err := s.PlexConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if connected || serverURL != "" || token != "" {
		t.Fatalf("got url=%q token=%q connected=%v, want all empty/false", serverURL, token, connected)
	}
}

func TestSavePlexConnection_RoundTrips(t *testing.T) {
	s := openTestStore(t)
	if err := s.SavePlexConnection("http://plex.local:32400", "tok-abc"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	serverURL, token, connected, err := s.PlexConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected {
		t.Error("expected connected = true")
	}
	if serverURL != "http://plex.local:32400" || token != "tok-abc" {
		t.Errorf("got url=%q token=%q", serverURL, token)
	}
}

func TestSavePlexConnection_OverwritesPrevious(t *testing.T) {
	s := openTestStore(t)
	if err := s.SavePlexConnection("http://old:32400", "old-tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.SavePlexConnection("http://new:32400", "new-tok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	serverURL, token, _, err := s.PlexConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if serverURL != "http://new:32400" || token != "new-tok" {
		t.Errorf("got url=%q token=%q, want the overwritten values", serverURL, token)
	}
}
