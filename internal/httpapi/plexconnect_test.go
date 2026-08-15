package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"chanarr/internal/plexlink"
)

func TestHandleStartLink(t *testing.T) {
	s := newTestServer(t)
	s.startLink = func(ctx context.Context, clientIdentifier string) (plexlink.Pin, error) {
		if clientIdentifier == "" {
			t.Error("expected a non-empty clientIdentifier")
		}
		return plexlink.Pin{ID: 42, Code: "ABCD"}, nil
	}

	rec, req := newJSONRequest("POST", "/api/plex/link/start", "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp startLinkResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.PinID != 42 || resp.Code != "ABCD" {
		t.Errorf("got %+v", resp)
	}
}

func TestHandleStartLink_UpstreamFailureIs502(t *testing.T) {
	s := newTestServer(t)
	s.startLink = func(ctx context.Context, clientIdentifier string) (plexlink.Pin, error) {
		return plexlink.Pin{}, errors.New("plex.tv unreachable")
	}
	rec, req := newJSONRequest("POST", "/api/plex/link/start", "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 502 {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestHandlePollLink_StillPending(t *testing.T) {
	s := newTestServer(t)
	s.startLink = func(ctx context.Context, clientIdentifier string) (plexlink.Pin, error) {
		return plexlink.Pin{ID: 42, Code: "ABCD"}, nil
	}
	s.pollLink = func(ctx context.Context, clientIdentifier string, pinID int64) (string, bool, error) {
		return "", false, nil
	}

	rec, req := newJSONRequest("POST", "/api/plex/link/start", "")
	s.Mux().ServeHTTP(rec, req)

	rec2, req2 := newJSONRequest("GET", "/api/plex/link/poll?pinId=42", "")
	s.Mux().ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec2.Code, rec2.Body.String())
	}
	var resp pollLinkResponse
	json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp.Linked {
		t.Error("expected linked = false")
	}
}

func TestHandlePollLink_UnknownPin(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("GET", "/api/plex/link/poll?pinId=999", "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandlePollLink_DoesNotRepollOncePreviouslyLinked(t *testing.T) {
	s := newTestServer(t)
	pollCount := 0
	s.startLink = func(ctx context.Context, clientIdentifier string) (plexlink.Pin, error) {
		return plexlink.Pin{ID: 42, Code: "ABCD"}, nil
	}
	s.pollLink = func(ctx context.Context, clientIdentifier string, pinID int64) (string, bool, error) {
		pollCount++
		return "secret-token", true, nil
	}

	rec, req := newJSONRequest("POST", "/api/plex/link/start", "")
	s.Mux().ServeHTTP(rec, req)

	for i := 0; i < 3; i++ {
		rec, req := newJSONRequest("GET", "/api/plex/link/poll?pinId=42", "")
		s.Mux().ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("poll %d: status = %d", i, rec.Code)
		}
	}
	if pollCount != 1 {
		t.Errorf("plexlink.PollLink called %d times, want exactly 1 (subsequent polls should use the cached result)", pollCount)
	}
}

func linkedTestServer(t *testing.T, pinID int64, token string) *Server {
	t.Helper()
	s := newTestServer(t)
	s.startLink = func(ctx context.Context, clientIdentifier string) (plexlink.Pin, error) {
		return plexlink.Pin{ID: pinID, Code: "ABCD"}, nil
	}
	s.pollLink = func(ctx context.Context, clientIdentifier string, id int64) (string, bool, error) {
		return token, true, nil
	}
	rec, req := newJSONRequest("POST", "/api/plex/link/start", "")
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("start link: status = %d", rec.Code)
	}
	rec2, req2 := newJSONRequest("GET", fmt.Sprintf("/api/plex/link/poll?pinId=%d", pinID), "")
	s.Mux().ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("poll link: status = %d", rec2.Code)
	}
	return s
}

func TestHandleSaveConnection(t *testing.T) {
	s := linkedTestServer(t, 42, "secret-token")

	rec, req := newJSONRequest("POST", "/api/plex/connection", `{"pinId":42,"serverUrl":"http://plex.local:32400"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	serverURL, token, connected, err := s.store.PlexConnection()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !connected || serverURL != "http://plex.local:32400" || token != "secret-token" {
		t.Errorf("got url=%q token=%q connected=%v", serverURL, token, connected)
	}
}

func TestHandleSaveConnection_TokenNeverLeavesServer(t *testing.T) {
	s := linkedTestServer(t, 42, "super-secret-token")

	rec, req := newJSONRequest("POST", "/api/plex/connection", `{"pinId":42,"serverUrl":"http://plex.local:32400"}`)
	s.Mux().ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "super-secret-token") {
		t.Error("the raw Plex token must never appear in an HTTP response body")
	}

	rec2, req2 := newJSONRequest("GET", "/api/plex/connection", "")
	s.Mux().ServeHTTP(rec2, req2)
	if strings.Contains(rec2.Body.String(), "super-secret-token") {
		t.Error("GET /api/plex/connection must never return the raw token")
	}
}

func TestHandleSaveConnection_BeforeLinkingIsConflict(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("POST", "/api/plex/connection", `{"pinId":999,"serverUrl":"http://x"}`)
	s.Mux().ServeHTTP(rec, req)
	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestHandleGetConnection_DisconnectedInitially(t *testing.T) {
	s := newTestServer(t)
	rec, req := newJSONRequest("GET", "/api/plex/connection", "")
	s.Mux().ServeHTTP(rec, req)
	var resp connectionResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Connected {
		t.Error("expected connected = false initially")
	}
}
