package guide

import (
	"encoding/xml"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"chanarr/internal/schedule"
)

func TestHandler_ServesXML(t *testing.T) {
	provider := func() ([]ChannelSchedule, error) {
		return []ChannelSchedule{{
			Channel: schedule.Channel{Number: "1", Name: "X"},
			Epoch: schedule.Epoch{
				Start: time.Now(),
				Items: []schedule.PlaylistItem{{Path: "/a.mkv", Duration: time.Hour}},
			},
		}}, nil
	}

	req := httptest.NewRequest("GET", "/epg.xml", nil)
	rec := httptest.NewRecorder()
	Handler(provider)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("Content-Type = %q, want application/xml", ct)
	}
	var doc xmltvDoc
	if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("response is not well-formed XML: %v", err)
	}
}

func TestHandler_ProviderErrorIs500(t *testing.T) {
	provider := func() ([]ChannelSchedule, error) {
		return nil, errors.New("boom")
	}

	req := httptest.NewRequest("GET", "/epg.xml", nil)
	rec := httptest.NewRecorder()
	Handler(provider)(rec, req)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHandler_EmptyChannelsIsValidEmptyGuide(t *testing.T) {
	provider := func() ([]ChannelSchedule, error) { return nil, nil }

	req := httptest.NewRequest("GET", "/epg.xml", nil)
	rec := httptest.NewRecorder()
	Handler(provider)(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc xmltvDoc
	if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("response is not well-formed XML: %v", err)
	}
}
