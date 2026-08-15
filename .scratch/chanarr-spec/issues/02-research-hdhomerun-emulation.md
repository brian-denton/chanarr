# Research: HDHomeRun emulation protocol for Plex

Type: research
Status: resolved

## Question

Exactly what must chanarr expose for Plex Live TV & DVR to detect and tune it as an HDHomeRun tuner? Endpoints (`discover.json`, `lineup.json`, `lineup_status.json`, others), device ID/model rules, SSDP discovery vs manual-IP entry, tuner-count semantics, stream URL format Plex requests, and known gotchas — as implemented by Threadfin, xTeVe, dizqueTV/Tunarr, and antennas.

## Answer

Full findings with example JSON and source links: [assets/research-hdhomerun-emulation.md](../assets/research-hdhomerun-emulation.md)

Summary:

- **Three GET endpoints are the entire required surface**: `/discover.json`, `/lineup.json`, `/lineup_status.json` (static stub). `/device.xml` recommended; `lineup.post` unnecessary.
- **Plex validates almost nothing in discover.json.** DeviceID needs no hex/checksum format (dizqueTV ships the string `"dizqueTV"`) but must be unique and stable — Plex keys the DVR to it, and changing it orphans the setup. DeviceAuth can be empty. Use the conventional `ModelNumber: "HDTC-2US"` / `FirmwareName: "hdhomeruntc_atsc"` identity. BaseURL/LineupURL must be built from the request Host header, never localhost.
- **Manual IP:port entry is the reliable discovery path** (Plex fetches `http://ip:port/discover.json`); SSDP is an optional nicety that breaks in Docker bridge networks. Real HDHR discovery is proprietary UDP :65001, which nobody emulates. Plex UI quirk: manually entered tuners may show no tile but Continue becomes clickable.
- **TunerCount is enforced client-side by Plex** (cached at setup; changes need re-add). Server should also enforce with HTTP 503 + `X-HDHomeRun-Error: 805` when saturated.
- **Stream URLs from lineup.json are replayed verbatim** — any clean path works; avoid query strings (Tunarr regression), suffix `.ts`, serve unbounded chunked `video/mp2t`, start emitting TS bytes immediately or Plex "could not tune channel", and treat abrupt disconnects as normal teardown (kill ffmpeg promptly).
- Other gotchas: empty lineup breaks setup (return placeholder channel); GuideNumber (string) must match the XMLTV channel numbers chanarr emits, which buys auto-matching; endpoints cannot be authenticated (network-level access control only).
