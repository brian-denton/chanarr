# chanarr v1 spec — wayfinder map

**CLOSED 2026-08-15 — destination reached: [spec.md](spec.md)**

Label: wayfinder:map
Tracker: local markdown (tickets in `issues/`, findings in `assets/`)

## Destination

A complete v1 spec + locked architecture decisions for **chanarr** — a single-binary Go app (embedded React UI, SQLite) that turns folders of media into looping virtual-timeline TV channels Plex detects via HDHomeRun tuner emulation + XMLTV. The map is done when implementation sessions can build v1 without anything left to decide. Planning only — this map does not build the app.

## Notes

- Domain: home media / IPTV / Plex Live TV & DVR.
- Skills: grilling tickets → /grilling + /domain-modeling; prototype tickets → /prototype; research tickets → /research subagents.
- North star requirement: folder → channel playing in Plex in under 10 minutes, no docs.
- Design lens for every decision: single-binary simplicity beats configurability (the anti-dispatcharr stance).
- Research findings live in `assets/research-<slug>.md`, linked from their ticket.

## Decisions so far

- [Founding decisions](issues/01-founding-decisions.md) — destination, Plex-via-tuner-emulation, virtual timeline, remux-first, v1 scope, single binary, <10-min north star
- [Research: HDHomeRun emulation protocol for Plex](issues/02-research-hdhomerun-emulation.md) — three GET endpoints suffice (discover/lineup/lineup_status), loose validation but DeviceID must be stable; design for manual IP:port entry (not SSDP/UDP discovery); TunerCount cached by Plex at setup; stream URLs replayed verbatim (no query strings), emit bytes immediately, no auth possible
- [Research: Plex guide data via XMLTV](issues/03-research-plex-guide-xmltv.md) — guide URL entered at DVR setup (Plex Pass, PMS-reachable); auto-map via GuideNumber ↔ XMLTV id/display-name trio; Plex's own re-poll is ~daily/unreliable → push `reloadGuide` with X-Plex-Token after each regen (needs optional Plex-connection setting); rolling 12h horizon regenerated every 4h; streamer and guide must share one deterministic timeline function
- [Research: continuous MPEG-TS stream pipeline](issues/04-research-stream-pipeline.md) — nobody runs one long-lived ffmpeg: one ffmpeg per program item + continuity layer (concat-pipe or per-channel HLS session remuxed to TS); mid-file join = input `-ss` (keyframe-glitchy under copy); PTS continuity via concat rewrite or chained `-output_ts_offset`; remux OK only on full codec/res/fps/audio match; remux ≈ free, software transcode ≈ 1 core per 1080p channel
- [Research: prior art survey](issues/05-research-prior-art.md) — HDHR+XMLTV route validated; steal from dizqueTV/Tunarr/ErsatzTV (zlib), ideas-only from dispatcharr/tvheadend (AGPL/GPL); dispatcharr's pain = IPTV-subscription focus + Redis/Postgres/Celery sprawl; single-binary lane is unoccupied

- [Task: verify Plex Pass + server version](issues/06-task-verify-plex-pass.md) — Plex Pass active; PMS on another LAN box (version TBD at prototype); chanarr will run as Docker beside Plex → manual IP entry + Host-header-derived URLs are mandatory spec requirements

- [Prototype: walking skeleton — Plex tunes one looping channel](issues/07-prototype-walking-skeleton.md) — confirmed: real Plex detects, sets up, and plays the minimal Go emulation (3 endpoints + XMLTV + per-request ffmpeg concat copy); manual IP entry and Host-header URLs proven; heterogeneous-media boundaries remain the open pipeline question

- [Grilling: virtual-timeline scheduling domain model](issues/08-grilling-scheduling-model.md) — pure function `ProgramAt(channel, t) → Airing` shared by streamer + guide (no persisted schedule); any playlist edit stamps a new immutable Epoch; shuffle fixed per epoch; filler-padded gaps; durations ffprobed once at epoch creation; one Channel = one recursively-scanned folder. Vocabulary: [CONTEXT.md](../../CONTEXT.md); design rationale: [ADR 0001](../../docs/adr/0001-epoch-based-pure-timeline-function.md)

- [Grilling: metadata sources & guide richness](issues/09-grilling-metadata-guide.md) — v1 metadata is filename-derived only (SxxExx regex parse, filename fallback); online TVDB/TMDB deferred; logo auto-detected from local convention file + manual override; Plex connection for guide push is optional/prompted via PIN-link flow, never required at onboarding ([ADR 0002](../../docs/adr/0002-optional-prompted-plex-connection.md)); guide defaults fixed at 12h horizon/4h refresh; always emit `<category>Series</category>`, omit `<rating>`

- [Prototype: channel management UI](issues/10-prototype-management-ui.md) — Variant A (wizard-first onboarding, single-column list, full-screen edit panel, dismissible Plex-connect banner) chosen over dashboard-grid and live-guide-grid alternatives; UI must support light + dark mode (system preference default, manual toggle). Full variant set kept at [assets/ui-prototype/index.html](../assets/ui-prototype/index.html)
- [Task: assemble the v1 spec](issues/11-task-assemble-spec.md) — [spec.md](spec.md) written, reviewed, and confirmed with the user; remaining minor fog (rescan cadence, TunerCount default, Docker networking, error handling) resolved inline as v1 defaults during assembly

## Not yet specified

None — all remaining items were resolved as v1 defaults during [spec assembly](issues/11-task-assemble-spec.md) (library rescan cadence, TunerCount, SQLite schema deferred to implementation, error/observability approach, onboarding detail, Docker networking guidance).

## Out of scope

Ruled out by [Founding decisions](issues/01-founding-decisions.md): multi-user auth; bumpers/interstitials between episodes; hardware transcoding; non-Plex clients as first-class targets (generic M3U noted as cheap add only); smart scheduling beyond a per-channel shuffle/in-order toggle.
