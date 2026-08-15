package tuner

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/discover.json", nil)
	req.Host = "192.168.1.148:5004"
	rec := httptest.NewRecorder()

	DiscoverHandler("test-device-id")(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	wantBase := "http://192.168.1.148:5004"
	if got["BaseURL"] != wantBase {
		t.Errorf("BaseURL = %v, want %v", got["BaseURL"], wantBase)
	}
	if got["LineupURL"] != wantBase+"/lineup.json" {
		t.Errorf("LineupURL = %v, want %v", got["LineupURL"], wantBase+"/lineup.json")
	}
	if got["DeviceID"] != "test-device-id" {
		t.Errorf("DeviceID = %v, want test-device-id", got["DeviceID"])
	}
	if got["TunerCount"] != float64(TunerCount) {
		t.Errorf("TunerCount = %v, want %v", got["TunerCount"], TunerCount)
	}
	// DeviceAuth must be present (Plex expects the field) even though
	// chanarr never populates it — see research-hdhomerun-emulation.md.
	if _, ok := got["DeviceAuth"]; !ok {
		t.Error("DeviceAuth field missing")
	}
}

func TestDiscoverHandler_BaseURLTracksHostPerRequest(t *testing.T) {
	// Regression guard: BaseURL must never be hardcoded/cached — it has to
	// reflect whatever Host a given request actually arrived on (critical
	// for the Docker-beside-Plex deployment, spec.md §1).
	hosts := []string{"192.168.1.148:5004", "chanarr.local:5004", "10.0.0.5:9999"}
	for _, host := range hosts {
		req := httptest.NewRequest("GET", "/discover.json", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		DiscoverHandler("test-device-id")(rec, req)

		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON for host %q: %v", host, err)
		}
		want := "http://" + host
		if got["BaseURL"] != want {
			t.Errorf("host %q: BaseURL = %v, want %v", host, got["BaseURL"], want)
		}
	}
}

func TestDiscoverHandler_UsesGivenDeviceID(t *testing.T) {
	// Regression guard for this handler's whole point: the deviceID must
	// come from the caller (internal/store), not a fixed constant.
	req := httptest.NewRequest("GET", "/discover.json", nil)
	req.Host = "192.168.1.148:5004"
	rec := httptest.NewRecorder()
	DiscoverHandler("chanarr-abc123")(rec, req)

	var got map[string]any
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["DeviceID"] != "chanarr-abc123" {
		t.Errorf("DeviceID = %v, want chanarr-abc123", got["DeviceID"])
	}
}

func TestLineupHandler_EmptyProviderReturnsPlaceholder(t *testing.T) {
	// Plex's setup flow breaks on a genuinely empty lineup array — this is
	// a hard requirement, not a nicety.
	handler := LineupHandler(func() []LineupEntry { return nil })

	req := httptest.NewRequest("GET", "/lineup.json", nil)
	req.Host = "192.168.1.148:5004"
	rec := httptest.NewRecorder()
	handler(rec, req)

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want exactly 1 placeholder", len(got))
	}
	if got[0]["GuideNumber"] != "1" {
		t.Errorf("placeholder GuideNumber = %v, want \"1\"", got[0]["GuideNumber"])
	}
}

func TestLineupHandler_MapsEntriesWithoutQueryStrings(t *testing.T) {
	provider := func() []LineupEntry {
		return []LineupEntry{
			{GuideNumber: "1", GuideName: "Classic Sitcoms"},
			{GuideNumber: "2", GuideName: "90s Cartoons"},
		}
	}
	handler := LineupHandler(provider)

	req := httptest.NewRequest("GET", "/lineup.json", nil)
	req.Host = "192.168.1.148:5004"
	rec := httptest.NewRecorder()
	handler(rec, req)

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	for i, want := range []struct{ number, name, url string }{
		{"1", "Classic Sitcoms", "http://192.168.1.148:5004/stream/1"},
		{"2", "90s Cartoons", "http://192.168.1.148:5004/stream/2"},
	} {
		if got[i]["GuideNumber"] != want.number {
			t.Errorf("entry %d GuideNumber = %v, want %v", i, got[i]["GuideNumber"], want.number)
		}
		if got[i]["GuideName"] != want.name {
			t.Errorf("entry %d GuideName = %v, want %v", i, got[i]["GuideName"], want.name)
		}
		url, _ := got[i]["URL"].(string)
		if url != want.url {
			t.Errorf("entry %d URL = %v, want %v", i, url, want.url)
		}
		// Stream URLs are replayed verbatim by Plex — query strings are a
		// documented Tunarr footgun (research-hdhomerun-emulation.md).
		if strings.Contains(url, "?") {
			t.Errorf("entry %d URL contains a query string: %v", i, url)
		}
	}
}

func TestLineupStatusHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/lineup_status.json", nil)
	rec := httptest.NewRecorder()

	LineupStatusHandler(rec, req)

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["ScanInProgress"] != float64(0) {
		t.Errorf("ScanInProgress = %v, want 0", got["ScanInProgress"])
	}
	if got["ScanPossible"] != float64(1) {
		t.Errorf("ScanPossible = %v, want 1", got["ScanPossible"])
	}
}

func TestDeviceXMLHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/device.xml", nil)
	req.Host = "192.168.1.148:5004"
	rec := httptest.NewRecorder()

	DeviceXMLHandler("test-device-id")(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("Content-Type = %q, want application/xml", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<URLBase>http://192.168.1.148:5004</URLBase>") {
		t.Errorf("body missing expected URLBase, got: %s", body)
	}
	if !strings.Contains(body, "<UDN>uuid:test-device-id</UDN>") {
		t.Errorf("body missing expected UDN, got: %s", body)
	}
}
