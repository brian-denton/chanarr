package plexlink

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withMockPlexTV(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = orig })
}

func TestNewClientIdentifier_UniqueAndNonEmpty(t *testing.T) {
	a, err := NewClientIdentifier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == "" {
		t.Fatal("expected a non-empty identifier")
	}
	b, err := NewClientIdentifier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Fatalf("expected distinct identifiers, got %q twice", a)
	}
}

func TestStartLink(t *testing.T) {
	var gotMethod, gotPath, gotClientID, gotAccept, gotContentType string
	withMockPlexTV(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotClientID = r.Header.Get("X-Plex-Client-Identifier")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":123456,"code":"ABCD","authToken":null}`))
	})

	pin, err := StartLink(context.Background(), "client-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pin.ID != 123456 || pin.Code != "ABCD" {
		t.Fatalf("got %+v, want {ID:123456 Code:ABCD}", pin)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/v2/pins" {
		t.Errorf("path = %q, want /api/v2/pins", gotPath)
	}
	if gotClientID != "client-abc" {
		t.Errorf("X-Plex-Client-Identifier = %q, want client-abc", gotClientID)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", gotContentType)
	}
}

func TestStartLink_NonOKStatus(t *testing.T) {
	withMockPlexTV(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := StartLink(context.Background(), "client-abc")
	if err == nil {
		t.Fatal("expected an error for a 401 response, got nil")
	}
}

func TestPollLink_StillPending(t *testing.T) {
	withMockPlexTV(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":123456,"code":"ABCD","authToken":null}`))
	})

	token, linked, err := PollLink(context.Background(), "client-abc", 123456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if linked {
		t.Error("expected linked = false while authToken is null")
	}
	if token != "" {
		t.Errorf("expected empty token, got %q", token)
	}
}

func TestPollLink_Completed(t *testing.T) {
	var gotPath, gotClientID string
	withMockPlexTV(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotClientID = r.Header.Get("X-Plex-Client-Identifier")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":123456,"code":"ABCD","authToken":"secret-token-xyz"}`))
	})

	token, linked, err := PollLink(context.Background(), "client-abc", 123456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !linked {
		t.Error("expected linked = true once authToken is populated")
	}
	if token != "secret-token-xyz" {
		t.Errorf("token = %q, want secret-token-xyz", token)
	}
	if gotPath != "/api/v2/pins/123456" {
		t.Errorf("path = %q, want /api/v2/pins/123456", gotPath)
	}
	if gotClientID != "client-abc" {
		t.Errorf("X-Plex-Client-Identifier = %q, want client-abc (must match the identifier that created the pin)", gotClientID)
	}
}

func TestPollLink_NonOKStatus(t *testing.T) {
	withMockPlexTV(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, _, err := PollLink(context.Background(), "client-abc", 999)
	if err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
}
