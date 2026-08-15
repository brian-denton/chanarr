# chanarr v1 Spec

Turns folders of local media into looping virtual-timeline TV channels that Plex detects and plays via HDHomeRun tuner emulation. Assembled from the [wayfinder map](map.md) — every decision below traces to a resolved ticket.

**North star / acceptance test**: a user with an unconfigured chanarr and a Plex Pass can go from "point at a folder" to "channel playing in Plex" in under 10 minutes, without reading documentation.

## 1. Architecture

Single static Go binary; React UI built and embedded in the binary; SQLite for all persisted state; `ffmpeg`/`ffprobe` as the one external runtime dependency (checked for at startup, with a clear error if missing). Docker image is a thin wrapper around the same binary, **not** a separate deployment story. *(Founding decisions, [ADR 0002](../../docs/adr/0002-optional-prompted-plex-connection.md)'s sibling reasoning — no service sprawl, unlike dispatcharr's Django+Redis+Postgres+Celery stack, see [prior-art research](assets/research-prior-art.md).)*

**Docker networking default**: recommend `--network host` (or an explicit port-matched bridge mapping) in the shipped compose file. Every URL chanarr advertises (tuner BaseURL, stream URLs, guide URL) is derived from the inbound request's `Host` header — see §4 — so whatever address/port Plex actually uses to reach chanarr must be the same address/port chanarr sees in that header; host networking is the simplest way to guarantee that. *(v1 default; not user-configurable choice, just documented deployment guidance.)*

## 2. Domain model

Full glossary: [CONTEXT.md](../../CONTEXT.md). Core entities:

- **Channel** — one configured source folder (scanned recursively) + settings (shuffle on/off, seed), exposed to Plex as one tuner lineup entry.
- **Epoch** — an immutable snapshot of a Channel's playlist (ordered items + cached ffprobed durations + shuffle order if applicable). Any playlist-membership or ordering change stamps a new epoch; `epochStart` = creation time, no user-settable anchor in v1.
- **Playlist Item** — one file's membership in an epoch: file path, cached duration, position.
- **Airing** — the result of `ProgramAt(channel, t) → Airing`: a Playlist Item paired with the concrete start/end time it occupies. The single unit both the streamer and the XMLTV writer consume.

**Architectural decision**: `ProgramAt` is a **pure function**, not a persisted/regenerated schedule — see [ADR 0001](../../docs/adr/0001-epoch-based-pure-timeline-function.md). This structurally guarantees the live stream and the published guide can never disagree about what's airing.

Failure handling: when ffmpeg can't produce an item's expected duration or a file goes missing, that Airing is filler/error-screen padded to its expected duration — all *future* Airing boundaries stay stable regardless of runtime playback failures.

## 3. Library scanning

*(Resolved during spec assembly — a v1-default implementation detail, not a product decision requiring its own ticket.)* Each channel's folder is rescanned on a periodic timer (default **every 5 minutes**) plus a manual "rescan now" action in the UI. No filesystem-watcher dependency in v1 (inotify/FSEvents behave inconsistently across Docker bind mounts — polling is simpler and matches the single-binary-simplicity lens). A rescan that detects any membership/ordering change stamps a new Epoch per §2.

## 4. Plex integration — HDHomeRun emulation

Full detail: [research-hdhomerun-emulation.md](assets/research-hdhomerun-emulation.md); proven end-to-end against a real Plex server in the [walking-skeleton prototype](assets/walking-skeleton/).

- Three endpoints suffice: `GET /discover.json`, `GET /lineup.json`, `GET /lineup_status.json` (static stub). `/device.xml` included for good measure; `lineup.post` not needed.
- `DeviceID` must be stable across restarts (persisted in SQLite) but needs no Silicondust checksum format — any unique string works. `ModelNumber: "HDTC-2US"`, `FirmwareName: "hdhomeruntc_atsc"` (conventional values, not validated by Plex).
- **BaseURL, LineupURL, and every stream URL are derived from the request's `Host` header** — never hardcoded or container-internal.
- **Design for manual IP:port tuner entry**, not SSDP discovery — real HDHomeRun discovery is a proprietary UDP protocol nobody emulates, and SSDP is unreliable under Docker bridge networking anyway (§1).
- `TunerCount` default: **4** (v1 default, not exposed as onboarding config). Plex caches this at tuner setup — changing it later requires the user to re-add the tuner in Plex, so the spec's default should be generous rather than tuned per-install. When saturated, return `503` + header `X-HDHomeRun-Error: 805`.
- Stream URLs are replayed verbatim by Plex — no query strings. Begin emitting TS bytes immediately on tune to avoid Plex's "could not tune channel" timeout. No authentication is possible on tuner endpoints (Plex doesn't support it).
- Empty lineups break Plex's setup flow — always return at least a placeholder channel if none are configured yet.

## 5. Plex integration — XMLTV guide

Full detail: [research-plex-guide-xmltv.md](assets/research-plex-guide-xmltv.md).

- Guide is served from the same host/port as the tuner, e.g. `/epg.xml`. The user enters this URL in Plex's DVR setup ("use an XMLTV guide instead").
- Channel mapping: XMLTV `<channel id>` = the channel number; emit the `"{number} {name}"` / `"{number}"` / `"{name}"` display-name trio so Plex auto-maps without manual intervention.
- **Horizon: 12 hours, regenerated every 4 hours** (fixed v1 defaults, prior-art convention — 3x margin against Plex showing "unknown airing" if a regen is late).
- Fields emitted: `title`, `sub-title` (parsed episode title if available), `desc`, `episode-num` (both `xmltv_ns` and `onscreen` systems — season component is mandatory for Plex to render "SxxExx"), `icon`, `<category lang="en">Series</category>` always, `<previously-shown/>` always. **`<rating>` omitted** — chanarr has no rating data and an absent rating is more honest than a fabricated one.
- Times in UTC (`yyyyMMddHHmmss +0000`).
- Because `ProgramAt` (§2) is pure and shared, the guide writer just calls it repeatedly, walking forward to each Airing's end time until the horizon is covered — the same primitive the streamer uses at tune-in.

## 6. Plex connection (guide push)

Plex's own XMLTV re-poll is roughly daily and unreliable; a fresh guide needs an authenticated `reloadGuide` push. Per [ADR 0002](../../docs/adr/0002-optional-prompted-plex-connection.md): **this is optional and prompted, never required at onboarding.** A channel works and plays without it. After onboarding completes, the UI surfaces a dismissible "connect Plex for instant guide updates" prompt using Plex's **PIN-based link flow** (`plex.tv/link`) — the user is shown a short code and enters it at plex.tv; chanarr polls for the resulting token. No manually-copied `X-Plex-Token`.

## 7. Streaming pipeline

Full detail: [research-stream-pipeline.md](assets/research-stream-pipeline.md); the core mechanism proven live: [walking-skeleton prototype](assets/walking-skeleton/).

- One ffmpeg process per program item, never one long-lived process across heterogeneous files. Continuity via `-f concat` piping (proven approach) — a fresh continuity layer if per-item boundary quality needs improve post-v1 (chained `-output_ts_offset` is the documented alternative).
- **Remux-by-default** (`-c copy`): valid only when a file's codec/profile/level, resolution, fps, and audio codec/channels/rate all match the channel's declared parameters and no filter is needed. Any mismatch flips that item to transcode.
- Tune-in mid-file: input-side `-ss` computed from the Airing's start time. Frame-accurate under transcode; keyframe-granular (occasionally a few seconds off) under remux — accepted TV-grade behavior, not a bug.
- Lifecycle: spawn ffmpeg per tuner request, kill on client disconnect. Filler/error-screen stream generator so the TS never dies on an item failure.
- Cost: remux is near-free (~single-digit % of a core); software transcode costs roughly one core per concurrent 1080p channel. No hardware-encode support in v1 (out of scope, §11).

## 8. Metadata

Full detail: [Metadata & guide richness ticket](issues/09-grilling-metadata-guide.md).

- Episode identity: parse `SxxExx` from filenames (Sonarr/Plex convention) via regex. When it matches, populate real season/episode numbers. When it doesn't, the Airing's title falls back to the filename, stripped of extension.
- **No online metadata (TVDB/TMDB) in v1** — filename parsing meets Plex's rendering requirements without an account, API key, or network call. Clean additive v1.1 feature.
- Ordering (non-shuffle channels): lexical path/filename sort — sufficient because it doesn't require parsing, and correctly orders any sanely-named library.
- Channel logo: auto-detect conventional local files (`poster.jpg`/`folder.jpg`) in the channel's folder; manual upload as an override in the UI.

## 9. UI

Winning direction: **Variant A — wizard-first**, chosen over a dashboard-grid and a live-guide-grid alternative. Full three-variant prototype (primary source, kept for reference — no git history yet to branch on): [assets/ui-prototype/index.html](assets/ui-prototype/index.html).

- **Onboarding**: full-screen step flow — enter/browse a folder path → scan result (episode count, detected name) → create. No Plex credentials requested at this stage.
- **Home**: minimal single-column list of configured channels (number, name/logo, "now playing").
- **Channel edit**: full-screen panel — name, channel number, logo (auto-detected indicator + upload override), shuffle toggle, delete.
- **Plex connect**: dismissible banner, using the PIN-link flow from §6, never blocking.
- **Theming**: light and dark mode both required; default to OS/browser system preference, with a manual override toggle. *(Surfaced by the user during UI review — standard pattern, no open sub-question.)*

## 10. Storage

SQLite. Entities needed: Channel, Epoch (+ its cached Playlist Items and durations), and whatever persists the stable tuner `DeviceID` and the optional Plex connection (server URL + token). Exact schema/migrations are implementation detail, not a spec blocker — the domain model in §2/[CONTEXT.md](../../CONTEXT.md) fully describes what must be representable.

## 11. Error handling & observability

*(v1 default, resolved during spec assembly.)* Structured logs to stdout (single-binary simplicity — no external telemetry, no log aggregation dependency in v1). Playback failures are absorbed by the filler/error-screen mechanism (§2, §7) rather than surfaced as hard errors to the viewer. UI surfaces scan/tuner/stream errors inline where relevant (e.g., "ffmpeg not found" at startup, per §1).

## 12. Out of scope for v1

Per [founding decisions](issues/01-founding-decisions.md): multi-user auth; bumpers/interstitials between episodes; hardware-accelerated transcoding; non-Plex clients as a first-class target (generic M3U output is a cheap incidental add, not a v1 goal); smart/rule-based scheduling beyond the per-channel shuffle/in-order toggle.

## 13. Deferred / v1.1 candidates

Online metadata lookup (TVDB/TMDB) for real titles/descriptions/artwork (§8); multi-folder-per-channel composition; user-settable epoch/channel anchor times; configurable guide horizon/refresh cadence; hardware transcoding.
