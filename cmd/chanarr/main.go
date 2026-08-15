// chanarr — turns folders of local media into looping virtual-timeline TV
// channels Plex detects via HDHomeRun tuner emulation. See .scratch/chanarr-spec/spec.md.
package main

import (
	"log"
	"net/http"

	"chanarr/internal/config"
	"chanarr/internal/guide"
	"chanarr/internal/httpapi"
	"chanarr/internal/stream"
	"chanarr/internal/tuner"
)

func main() {
	if err := config.CheckFFmpeg(); err != nil {
		log.Fatal(err)
	}
	cfg := config.Load()

	// TODO: replace with internal/store-backed lineup/guide once channel
	// persistence exists.
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
