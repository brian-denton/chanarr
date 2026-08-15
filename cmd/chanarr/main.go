// chanarr — turns folders of local media into looping virtual-timeline TV
// channels Plex detects via HDHomeRun tuner emulation. See .scratch/chanarr-spec/spec.md.
package main

import (
	"log"
	"net/http"

	"chanarr/internal/config"
	"chanarr/internal/guide"
	"chanarr/internal/httpapi"
	"chanarr/internal/store"
	"chanarr/internal/stream"
	"chanarr/internal/tuner"
)

func main() {
	if err := config.CheckFFmpeg(); err != nil {
		log.Fatal(err)
	}
	cfg := config.Load()

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	deviceID, err := db.DeviceID()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("chanarr device ID:", deviceID)
	// TODO: internal/tuner.DeviceID is still a fixed const — wire this
	// store-generated deviceID into DiscoverHandler/DeviceXMLHandler
	// (mirroring LineupHandler's provider pattern) as a follow-up.

	// TODO: replace with db-backed lineup/guide providers once
	// internal/httpapi exposes channel CRUD against internal/store.
	emptyLineup := func() []tuner.LineupEntry { return nil }
	emptyGuide := func() ([]guide.ChannelSchedule, error) { return nil, nil }

	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", tuner.DiscoverHandler)
	mux.HandleFunc("/lineup.json", tuner.LineupHandler(emptyLineup))
	mux.HandleFunc("/lineup_status.json", tuner.LineupStatusHandler)
	mux.HandleFunc("/device.xml", tuner.DeviceXMLHandler)
	mux.HandleFunc("/epg.xml", guide.Handler(emptyGuide))
	mux.HandleFunc("/stream/", stream.Handler)
	mux.Handle("/api/", httpapi.Mux())
	// TODO: serve the embedded React build (web/dist) for everything else.

	log.Println("chanarr listening on", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}
