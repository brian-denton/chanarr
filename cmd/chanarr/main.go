// chanarr — turns folders of local media into looping virtual-timeline TV
// channels Plex detects via HDHomeRun tuner emulation. See .scratch/chanarr-spec/spec.md.
package main

import (
	"log"
	"net/http"

	"chanarr/internal/config"
	"chanarr/internal/guide"
	"chanarr/internal/httpapi"
	"chanarr/internal/schedule"
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

	lineupProvider := func() []tuner.LineupEntry {
		channels, err := db.Channels()
		if err != nil {
			log.Println("lineup: ", err)
			return nil
		}
		entries := make([]tuner.LineupEntry, len(channels))
		for i, c := range channels {
			entries[i] = tuner.LineupEntry{GuideNumber: c.Number, GuideName: c.Name}
		}
		return entries
	}
	guideProvider := func() ([]guide.ChannelSchedule, error) {
		channels, err := db.Channels()
		if err != nil {
			return nil, err
		}
		out := make([]guide.ChannelSchedule, 0, len(channels))
		for _, c := range channels {
			epoch, err := db.CurrentEpoch(c.ID)
			if err != nil {
				continue // no epoch yet (e.g. an empty folder) — omit from the guide
			}
			out = append(out, guide.ChannelSchedule{Channel: c, Epoch: epoch})
		}
		return out, nil
	}

	streamProvider := func(number string) (schedule.Channel, schedule.Epoch, error) {
		channels, err := db.Channels()
		if err != nil {
			return schedule.Channel{}, schedule.Epoch{}, err
		}
		for _, c := range channels {
			if c.Number == number {
				epoch, err := db.CurrentEpoch(c.ID)
				return c, epoch, err
			}
		}
		return schedule.Channel{}, schedule.Epoch{}, store.ErrNotFound
	}

	api := httpapi.NewServer(db, cfg.LogosDir)

	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", tuner.DiscoverHandler(deviceID))
	mux.HandleFunc("/lineup.json", tuner.LineupHandler(lineupProvider))
	mux.HandleFunc("/lineup_status.json", tuner.LineupStatusHandler)
	mux.HandleFunc("/device.xml", tuner.DeviceXMLHandler(deviceID))
	mux.HandleFunc("/epg.xml", guide.Handler(guideProvider))
	mux.HandleFunc("/stream/{number}", stream.Handler(streamProvider))
	mux.Handle("/api/", api.Mux())
	// TODO: serve the embedded React build (web/dist) for everything else.

	log.Println("chanarr listening on", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, mux))
}
