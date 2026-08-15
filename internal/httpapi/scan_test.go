package httpapi

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestHandleScan_SuggestsNameAndNumber(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv", "ep2.mkv", "ep3.mkv")

	rec, req := newJSONRequest("POST", "/api/scan", fmt.Sprintf(`{"folder":%q}`, dir))
	s.Mux().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp scanResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Number != "1" {
		t.Errorf("Number = %q, want 1 (no existing channels)", resp.Number)
	}
	if resp.HasLogo {
		t.Error("HasLogo = true, want false (no poster/folder file)")
	}
}

func TestHandleScan_SkipsUnprobeableFilesInCount(t *testing.T) {
	// newMediaFolder's fixture files aren't real media, so ffprobe fails
	// and library.Scan skips them — episodeCount should reflect that, not
	// error out.
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")

	rec, req := newJSONRequest("POST", "/api/scan", fmt.Sprintf(`{"folder":%q}`, dir))
	s.Mux().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp scanResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.EpisodeCount != 0 {
		t.Errorf("EpisodeCount = %d, want 0 (fixture files aren't probeable media)", resp.EpisodeCount)
	}
}

func TestHandleScan_DetectsLogo(t *testing.T) {
	s := newTestServer(t)
	dir := newMediaFolder(t, "ep1.mkv")
	mustWriteFile(t, dir+"/poster.jpg", "fake image bytes")

	rec, req := newJSONRequest("POST", "/api/scan", fmt.Sprintf(`{"folder":%q}`, dir))
	s.Mux().ServeHTTP(rec, req)

	var resp scanResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.HasLogo {
		t.Error("HasLogo = false, want true (poster.jpg present)")
	}
}

func TestHandleScan_MissingFolderField(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("POST", "/api/scan", `{}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleScan_NonexistentFolder(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("POST", "/api/scan", `{"folder":"/no/such/folder/chanarr-test"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
