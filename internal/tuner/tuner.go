// Package tuner emulates an HDHomeRun network tuner so Plex Live TV & DVR
// detects chanarr and tunes its channels. See spec.md §4 and
// .scratch/chanarr-spec/assets/research-hdhomerun-emulation.md — every
// endpoint here was proven against a real Plex server in the
// walking-skeleton prototype (.scratch/chanarr-spec/assets/walking-skeleton/).
//
// All advertised URLs are derived from the inbound request's Host header,
// never hardcoded or container-internal — required for the Docker-beside-Plex
// deployment (spec.md §1).
package tuner

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// DeviceID must be stable across restarts (persisted in internal/store) but
// needs no Silicondust checksum format — any unique string works. TODO:
// load from store instead of a fixed constant.
const DeviceID = "chanarr1"

const TunerCount = 4 // spec.md §4 default

// Lineup is the tuner's advertised channel list, in HDHomeRun lineup.json
// shape. TODO: back this with internal/library + internal/store instead of
// being supplied directly by the caller.
type LineupEntry struct {
	GuideNumber string
	GuideName   string
}

// LineupProvider supplies the current channel lineup. TODO: implement
// against internal/store once channel persistence exists.
type LineupProvider func() []LineupEntry

func baseURL(r *http.Request) string { return "http://" + r.Host }

func jsonResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// DiscoverHandler serves GET /discover.json.
func DiscoverHandler(w http.ResponseWriter, r *http.Request) {
	base := baseURL(r)
	jsonResponse(w, map[string]any{
		"FriendlyName":    "chanarr",
		"Manufacturer":    "Silicondust",
		"ModelNumber":     "HDTC-2US",
		"FirmwareName":    "hdhomeruntc_atsc",
		"FirmwareVersion": "20260101",
		"DeviceID":        DeviceID,
		"DeviceAuth":      "",
		"TunerCount":      TunerCount,
		"BaseURL":         base,
		"LineupURL":       base + "/lineup.json",
	})
}

// LineupHandler serves GET /lineup.json.
func LineupHandler(provider LineupProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := baseURL(r)
		entries := provider()
		out := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			out = append(out, map[string]any{
				"GuideNumber": e.GuideNumber,
				"GuideName":   e.GuideName,
				"URL":         base + "/stream/" + e.GuideNumber,
			})
		}
		// Plex's setup flow breaks on an empty lineup — always return at
		// least a placeholder if no channels are configured yet.
		if len(out) == 0 {
			out = append(out, map[string]any{
				"GuideNumber": "1",
				"GuideName":   "chanarr — no channels yet",
				"URL":         base + "/stream/1",
			})
		}
		jsonResponse(w, out)
	}
}

// LineupStatusHandler serves GET /lineup_status.json (static stub).
func LineupStatusHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]any{
		"ScanInProgress": 0, "ScanPossible": 1,
		"Source": "Cable", "SourceList": []string{"Cable"},
	})
}

// DeviceXMLHandler serves GET /device.xml.
func DeviceXMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0">`+
		`<specVersion><major>1</major><minor>0</minor></specVersion>`+
		`<URLBase>%s</URLBase><device>`+
		`<deviceType>urn:schemas-upnp-org:device-1-0:MediaServer:1</deviceType>`+
		`<friendlyName>chanarr</friendlyName><manufacturer>Silicondust</manufacturer>`+
		`<modelName>HDTC-2US</modelName><modelNumber>HDTC-2US</modelNumber>`+
		`<UDN>uuid:%s</UDN></device></root>`, baseURL(r), DeviceID)
}
