// Package guide generates the XMLTV guide chanarr serves to Plex and pushes
// reloadGuide refreshes via the optional Plex connection. See spec.md §5–§6
// and .scratch/chanarr-spec/assets/research-plex-guide-xmltv.md.
//
// Defaults: 12-hour horizon, regenerated every 4 hours (fixed in v1, not
// user-configurable). Always emits <category>Series</category>; omits
// <rating> (no data to report).
//
// Built on internal/schedule.ProgramAt, called repeatedly to walk forward
// to each Airing's end time until the horizon is covered — the same
// primitive the streamer uses at tune-in, so the guide can't drift out of
// sync with what's actually playing (docs/adr/0001).
package guide

import (
	"net/http"
	"time"
)

const (
	HorizonHours = 12
	RefreshHours = 4
)

// Handler serves GET /epg.xml.
func Handler(provider EpochProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channels, err := provider()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body, err := BuildXMLTV(channels, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write(body)
	}
}
