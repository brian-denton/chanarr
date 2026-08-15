// PROTOTYPE — chanarr walking skeleton. Throwaway; answers ticket 07:
// does real Plex detect an emulated HDHomeRun and play a looping ffmpeg stream?
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const port = "5004"

func base(r *http.Request) string { return "http://" + r.Host }

func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func main() {
	dir, _ := os.Getwd()

	http.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		log.Println("HIT /discover.json from", r.RemoteAddr)
		jsonOut(w, map[string]any{
			"FriendlyName":    "chanarr-proto",
			"Manufacturer":    "Silicondust",
			"ModelNumber":     "HDTC-2US",
			"FirmwareName":    "hdhomeruntc_atsc",
			"FirmwareVersion": "20200101",
			"DeviceID":        "chanarrproto1",
			"DeviceAuth":      "",
			"TunerCount":      2,
			"BaseURL":         base(r),
			"LineupURL":       base(r) + "/lineup.json",
		})
	})

	http.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		log.Println("HIT /lineup.json from", r.RemoteAddr)
		jsonOut(w, []map[string]any{{
			"GuideNumber": "1",
			"GuideName":   "Chanarr Proto",
			"URL":         base(r) + "/stream/1",
		}})
	})

	http.HandleFunc("/lineup_status.json", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, map[string]any{
			"ScanInProgress": 0, "ScanPossible": 1,
			"Source": "Cable", "SourceList": []string{"Cable"},
		})
	})

	http.HandleFunc("/device.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><specVersion><major>1</major><minor>0</minor></specVersion><URLBase>%s</URLBase><device><deviceType>urn:schemas-upnp-org:device-1-0:MediaServer:1</deviceType><friendlyName>chanarr-proto</friendlyName><manufacturer>Silicondust</manufacturer><modelName>HDTC-2US</modelName><modelNumber>HDTC-2US</modelNumber><UDN>uuid:chanarrproto1</UDN></device></root>`, base(r))
	})

	// Rolling guide: 1h back to +12h, hourly blocks, times in UTC.
	http.HandleFunc("/epg.xml", func(w http.ResponseWriter, r *http.Request) {
		log.Println("HIT /epg.xml from", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?><tv>`)
		fmt.Fprint(w, `<channel id="1"><display-name>1 Chanarr Proto</display-name><display-name>1</display-name><display-name>Chanarr Proto</display-name></channel>`)
		start := time.Now().UTC().Truncate(time.Hour).Add(-1 * time.Hour)
		for i := 0; i < 13; i++ {
			s, e := start.Add(time.Duration(i)*time.Hour), start.Add(time.Duration(i+1)*time.Hour)
			fmt.Fprintf(w, `<programme start="%s +0000" stop="%s +0000" channel="1"><title>Chanarr Test Loop</title><desc>Two looping test episodes.</desc></programme>`,
				s.Format("20060102150405"), e.Format("20060102150405"))
		}
		fmt.Fprint(w, `</tv>`)
	})

	http.HandleFunc("/stream/1", func(w http.ResponseWriter, r *http.Request) {
		log.Println("TUNE /stream/1 from", r.RemoteAddr)
		w.Header().Set("Content-Type", "video/mp2t")
		cmd := exec.CommandContext(r.Context(), "ffmpeg",
			"-hide_banner", "-loglevel", "error",
			"-re", "-stream_loop", "-1",
			"-fflags", "+genpts",
			"-f", "concat", "-safe", "0", "-i", dir+"/playlist.txt",
			"-c", "copy", "-f", "mpegts", "pipe:1")
		cmd.Stdout = w
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Println("ffmpeg ended:", err)
		}
		log.Println("UNTUNE /stream/1", r.RemoteAddr)
	})

	log.Println("chanarr-proto listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
