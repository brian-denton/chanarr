package store

import (
	"testing"

	"chanarr/internal/schedule"
)

func TestRelocateLogos(t *testing.T) {
	st := openTestStore(t)
	rel, err := st.SaveChannel(schedule.Channel{Number: "1", Name: "A", Folder: "/tv/a", Logo: "logos/1.png"})
	if err != nil {
		t.Fatal(err)
	}
	abs, err := st.SaveChannel(schedule.Channel{Number: "2", Name: "B", Folder: "/tv/b", Logo: "/media/tv/b/poster.jpg"})
	if err != nil {
		t.Fatal(err)
	}

	if err := st.RelocateLogos("/data/logos"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := st.Channel(rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Logo != "/data/logos/1.png" {
		t.Errorf("relative logo = %q, want re-anchored under the new dir", got.Logo)
	}
	got, err = st.Channel(abs.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Logo != "/media/tv/b/poster.jpg" {
		t.Errorf("absolute logo = %q, must be untouched", got.Logo)
	}
}
