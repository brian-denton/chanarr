# Task: verify Plex Pass + server version

Type: task
Status: resolved

## Question

Plex Live TV & DVR requires a Plex Pass subscription. Confirm the target Plex server has one, note the Plex Media Server version, and note the network topology (same box as chanarr? NAS? subnet) — facts the tuner-discovery and deployment decisions depend on.

## Answer

Verified with the user 2026-08-15:

- **Plex Pass: active** — Live TV & DVR is available; the tuner+XMLTV route is viable.
- **PMS location**: another PC/server on the LAN (not this Mac — nothing at localhost:32400, no GDM discovery response from here). **Version: unverified** — capture at the walking-skeleton prototype, which needs the real server anyway.
- **chanarr deployment target: Docker on the same box as Plex.** Implications the spec must carry: manual IP:port tuner entry is the designed path (SSDP unreliable under Docker bridge — per HDHomeRun research); BaseURL/guide URLs must derive from the request Host header, never container-internal addresses; PMS reaches chanarr's guide URL trivially (same host). The single-binary story remains for bare-metal users; Docker is the user's own primary mode.
