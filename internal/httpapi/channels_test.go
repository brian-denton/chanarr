package httpapi

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
)

func createChannel(t *testing.T, s *Server, folder, number, name string, shuffle bool) channelView {
	t.Helper()
	body := fmt.Sprintf(`{"number":%q,"name":%q,"folder":%q,"shuffle":%v}`, number, name, folder, shuffle)
	rec, req := newJSONRequest("POST", "/api/channels", body)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create channel: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var view channelView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return view
}

func TestHandleCreateChannel(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv", "ep2.mkv")

	view := createChannel(t, s, dir, "1", "Test Channel", false)
	if view.ID == 0 {
		t.Fatal("expected a non-zero ID")
	}
	if view.Number != "1" || view.Name != "Test Channel" || view.Folder != dir {
		t.Errorf("got %+v", view)
	}

	rec, req := newJSONRequest("GET", "/api/channels", "")
	s.Mux().ServeHTTP(rec, req)
	var list []channelView
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("got %d channels, want 1", len(list))
	}
}

func TestHandleCreateChannel_MissingFields(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("POST", "/api/channels", `{"name":"X"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleCreateChannel_AutoAssignsShuffleSeedWhenShuffled(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	view := createChannel(t, s, dir, "1", "X", true)
	if view.ShuffleSeed == 0 {
		t.Error("expected a non-zero auto-generated shuffle seed")
	}
}

func TestHandleCreateChannel_ShuffleSeedSerializedAsJSONStringForFullInt64Precision(t *testing.T) {
	// randomSeed() draws from the full int64 range (crypto/rand over 8
	// bytes), which routinely exceeds 2^53 — the largest integer a JS
	// float64 (and therefore any JSON number) can represent exactly. A
	// bare JSON number would get silently rounded by any JS client,
	// corrupting the seed on the very first round-trip. Encoding it as a
	// JSON string (`,string` tag) is what avoids that.
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")

	rec, req := newJSONRequest("POST", "/api/channels",
		fmt.Sprintf(`{"number":"1","name":"X","folder":%q,"shuffle":true}`, dir))
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, isString := raw["shuffleSeed"].(string); !isString {
		t.Fatalf("shuffleSeed = %#v (%T), want a JSON string", raw["shuffleSeed"], raw["shuffleSeed"])
	}
}

// requireFFmpeg gates the tests below that need library.Scan to actually
// produce non-empty, probeable items — same portability guard as
// internal/library's own tests.
func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not on PATH")
	}
}

func generateClip(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac",
		"-f", "mpegts", path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test clip: %v\n%s", err, out)
	}
}

func TestHandleCreateChannel_RealMediaProducesEpoch(t *testing.T) {
	requireFFmpeg(t)
	s := newTestServer(t)
	dir := t.TempDir()
	generateClip(t, filepath.Join(dir, "ep1.ts"))

	view := createChannel(t, s, dir, "1", "Real Channel", false)
	if view.EpisodeCount != 1 {
		t.Fatalf("EpisodeCount = %d, want 1", view.EpisodeCount)
	}
	if view.NowPlaying == "" {
		t.Error("expected NowPlaying to be populated from the real epoch")
	}
}

func TestHandleUpdateChannel_NameAndNumber(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "Old Name", false)

	rec, req := newJSONRequest("PUT", fmt.Sprintf("/api/channels/%d", created.ID),
		`{"number":"2","name":"New Name","shuffle":false,"shuffleSeed":"0"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var updated channelView
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Number != "2" || updated.Name != "New Name" {
		t.Errorf("got %+v", updated)
	}
}

func TestHandleUpdateChannel_ShuffleToggleStampsNewEpoch(t *testing.T) {
	requireFFmpeg(t)
	s := newTestServer(t)
	dir := t.TempDir()
	generateClip(t, filepath.Join(dir, "ep1.ts"))
	generateClip(t, filepath.Join(dir, "ep2.ts"))
	created := createChannel(t, s, dir, "1", "X", false)

	before, err := s.store.CurrentEpoch(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec, req := newJSONRequest("PUT", fmt.Sprintf("/api/channels/%d", created.ID),
		`{"number":"1","name":"X","shuffle":true,"shuffleSeed":"42"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	after, err := s.store.CurrentEpoch(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if after.ID == before.ID {
		t.Error("expected a new epoch to be stamped after toggling shuffle on")
	}
}

func TestHandleUpdateChannel_EnablingShuffleWithZeroSeedGetsAFreshOne(t *testing.T) {
	// Mirrors handleCreateChannel's auto-generation — without it, every
	// channel that gets shuffle turned on via an edit (rather than at
	// creation) would deterministically land on seed 0.
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)
	if created.ShuffleSeed != 0 {
		t.Fatalf("test fixture bug: expected a fresh non-shuffled channel to start at seed 0, got %d", created.ShuffleSeed)
	}

	rec, req := newJSONRequest("PUT", fmt.Sprintf("/api/channels/%d", created.ID),
		`{"number":"1","name":"X","shuffle":true,"shuffleSeed":"0"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var updated channelView
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.ShuffleSeed == 0 {
		t.Error("expected a freshly generated non-zero seed, got 0")
	}
}

func TestHandleUpdateChannel_PlainEditDoesNotStampNewEpoch(t *testing.T) {
	requireFFmpeg(t)
	s := newTestServer(t)
	dir := t.TempDir()
	generateClip(t, filepath.Join(dir, "ep1.ts"))
	created := createChannel(t, s, dir, "1", "X", false)

	before, err := s.store.CurrentEpoch(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec, req := newJSONRequest("PUT", fmt.Sprintf("/api/channels/%d", created.ID),
		`{"number":"1","name":"Renamed","shuffle":false,"shuffleSeed":"0"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	after, err := s.store.CurrentEpoch(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if after.ID != before.ID {
		t.Error("expected a plain name edit to leave the current epoch untouched")
	}
}

func TestHandleUpdateChannel_NotFound(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("PUT", "/api/channels/999", `{"number":"1","name":"X"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleDeleteChannel(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)

	rec, req := newJSONRequest("DELETE", fmt.Sprintf("/api/channels/%d", created.ID), "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	rec2, req2 := newJSONRequest("GET", "/api/channels", "")
	s.Mux().ServeHTTP(rec2, req2)
	var list []channelView
	json.Unmarshal(rec2.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("got %d channels after delete, want 0", len(list))
	}
}

func TestHandleDeleteChannel_NotFound(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("DELETE", "/api/channels/999", "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleRescanChannel_NoChange(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	created := createChannel(t, s, dir, "1", "X", false)

	rec, req := newJSONRequest("POST", fmt.Sprintf("/api/channels/%d/rescan", created.ID), "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp rescanResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Changed {
		t.Error("expected changed = false when nothing on disk changed")
	}
}

func TestHandleRescanChannel_DetectsNewFile(t *testing.T) {
	requireFFmpeg(t)
	s := newTestServer(t)
	dir := t.TempDir()
	generateClip(t, filepath.Join(dir, "ep1.ts"))
	created := createChannel(t, s, dir, "1", "X", false)

	before, err := s.store.CurrentEpoch(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	generateClip(t, filepath.Join(dir, "ep2.ts"))

	rec, req := newJSONRequest("POST", fmt.Sprintf("/api/channels/%d/rescan", created.ID), "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp rescanResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.Changed {
		t.Error("expected changed = true after adding a file")
	}
	if resp.EpisodeCount != 2 {
		t.Errorf("EpisodeCount = %d, want 2", resp.EpisodeCount)
	}

	after, err := s.store.CurrentEpoch(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if after.ID == before.ID {
		t.Error("expected a new epoch after a detected change")
	}
}

func TestHandleRescanChannel_NotFound(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("POST", "/api/channels/999/rescan", "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
