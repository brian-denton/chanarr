package guide

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListDVRs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/livetv/dvrs" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Plex-Token"); got != "tok123" {
			t.Errorf("X-Plex-Token = %q, want tok123", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"MediaContainer":{"Dvr":[
			{"key":"1","Device":{"key":"dev-1"}},
			{"key":"2","Device":{"key":"dev-2"}}
		]}}`))
	}))
	defer srv.Close()

	dvrs, err := ListDVRs(context.Background(), srv.URL, "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dvrs) != 2 {
		t.Fatalf("got %d DVRs, want 2", len(dvrs))
	}
	if dvrs[0].Key != "1" || dvrs[0].DeviceKey != "dev-1" {
		t.Errorf("dvrs[0] = %+v", dvrs[0])
	}
	if dvrs[1].Key != "2" || dvrs[1].DeviceKey != "dev-2" {
		t.Errorf("dvrs[1] = %+v", dvrs[1])
	}
}

func TestListDVRs_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := ListDVRs(context.Background(), srv.URL, "bad-token")
	if err == nil {
		t.Fatal("expected an error for a 401 response, got nil")
	}
}

func TestPushReload(t *testing.T) {
	var gotPath, gotMethod, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Plex-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := PushReload(context.Background(), srv.URL, "tok123", "dvr-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/livetv/dvrs/dvr-1/reloadGuide" {
		t.Errorf("path = %q, want /livetv/dvrs/dvr-1/reloadGuide", gotPath)
	}
	if gotToken != "tok123" {
		t.Errorf("X-Plex-Token = %q, want tok123", gotToken)
	}
}

func TestPushReload_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := PushReload(context.Background(), srv.URL, "tok123", "dvr-1")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}
