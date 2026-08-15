# Prior art survey: virtual-channel / tuner-emulation projects

Researched 2026-08-15 for chanarr. Projects: tvheadend, dispatcharr, dizqueTV, Tunarr, ErsatzTV.

## Summary table

| Project | Stack | Plex integration | Scheduling | Setup | License | Status (2026) |
|---|---|---|---|---|---|---|
| [tvheadend](https://github.com/tvheadend/tvheadend) | C, Linux | None native (needs tvhproxy HDHR shim) | Real broadcast DVR | configure/make or Docker | **GPL-3.0** | Active, different problem space |
| [dispatcharr](https://github.com/dispatcharr/dispatcharr) | Django + React + Redis + Postgres + Celery | HDHR emulation | DVR recordings, no virtual timeline | Docker (AIO or 4-container modular), then 8-step channel wiring | **AGPL-3.0** | Active |
| [dizqueTV](https://github.com/vexorian/dizquetv) | Node.js/Express + Bootstrap UI | HDHR spoof + XMLTV + M3U, Plex-only | Virtual timeline (pioneer) | Docker or pkg binary | MIT (pseudotv base) + **zlib** (improvements) | Maintenance mode, ~180 open issues |
| [Tunarr](https://github.com/chrisbenincasa/tunarr) | TypeScript/Node monorepo + React | HDHR spoof + XMLTV + M3U; Plex/Jellyfin/Emby/local | Virtual timeline + time-slot & random-slot tools, filler | Docker Compose (bundles ffmpeg) or bare binaries (**no ffmpeg bundled**) | **zlib** | Active; long beta, recent 1.0 migration docs |
| [ErsatzTV](https://github.com/ErsatzTV/ErsatzTV) | C# .NET (→ [Rust rewrite](https://github.com/ErsatzTV/next)) | HDHR emulation + XMLTV; Plex/Jellyfin/Emby/local sources | Deterministic playouts, 5 schedule types | Docker or install; needs its own ffmpeg builds | **zlib** (legacy), **MIT** (next) | C# repo now `ErsatzTV/legacy` (still updated); Rust `next` "VERY EARLY", experimental |

## Per-project findings

### tvheadend
TV streaming server/DVR for real broadcast hardware (DVB-C/S/T, ATSC, SAT>IP, IPTV inputs; HTSP/HTTP/SAT>IP outputs). C, web config UI, GPL-3.0. **No native Plex path** — users bolt on [tvhproxy](https://hub.docker.com/r/chvb/docker-tvhproxy) to spoof an HDHomeRun, which is itself evidence that HDHR emulation is the universal lingua franca for Plex Live TV. Not a channel-looper; relevant only as the deep end chanarr must not drift toward, and as protocol prior art. GPL-3: no code borrowing unless chanarr goes GPL.

### dispatcharr — why setup is complicated (chanarr's founding pain)
[Repo](https://github.com/dispatcharr/dispatcharr) · [Install docs](https://dispatcharr.github.io/Dispatcharr-Docs/installation/) · [Getting started](https://dispatcharr.github.io/Dispatcharr-Docs/getting-started/)

Dispatcharr is an **IPTV stream manager/proxy** (the grown-up Threadfin/xTeVe class): it ingests provider M3U playlists and EPG feeds, maps/filters channels, proxies streams, and presents an HDHR tuner to Plex. Diagnosis of the complexity, in order of importance:

1. **It solves a different problem.** Its core object is an external IPTV subscription, not local media. There is no "point at a folder" path — without an M3U source you can't start. A user who just wants "my Simpsons folder as a channel" is in the wrong tool, but nothing tells them that.
2. **Jargon-first onboarding.** The getting-started flow is ~8 distinct steps/concepts before a channel plays in Plex: add M3U account (choose "Xtream Codes or Standard M3U"), select M3U groups, add EPG source, create channel, copy HDHR URL, set up Plex tuner, add EPG URL to Plex, then manually map EPG entries that didn't auto-match. Auto-match is unreliable enough that [EPG Automatch is an open feature request (#493)](https://github.com/Dispatcharr/Dispatcharr/issues/493): on M3U refresh, channels are recreated but EPG assignments are lost.
3. **Service sprawl.** Django backend + React frontend + **Redis + PostgreSQL + Celery** task queue. The "all-in-one" container hides Redis/Postgres inside one image (still needs `DISPATCHARR_ENV=aio`, `REDIS_HOST`, `CELERY_BROKER_URL` env vars); "modular" mode is four containers with DB credentials plumbing. Even the happy path exposes infrastructure vocabulary.
4. **Guide/channel state is fragile** across playlist refreshes (see #493), so upkeep is ongoing, not one-time.

License **AGPL-3.0** — ideas only, never code.

### dizqueTV
[Repo](https://github.com/vexorian/dizquetv). Node.js/Express fork of the abandoned pseudotv-plex. Plex-only. Proved the whole model chanarr is built on: **HDHR spoofing** ("mocking a HDHR server"), XMLTV written to `.dizquetv/xmltv.xml`, M3U output, and the **virtual timeline** — channels "have a life of their own and act as if they continued playing when you weren't watching," with an optional on-demand mode. Shipped as Docker *or* a single `pkg`-bundled binary — closest prior art to chanarr's deployment story. Weaknesses: maintenance mode, dated UI, no automatic library watching (manual channel updates), ~180 open issues. Licensing: original pseudotv code MIT, dizqueTV improvements **zlib** — borrowable.

### Tunarr
[Repo](https://github.com/chrisbenincasa/tunarr) · [Docs](https://tunarr.com/). Started as a dizqueTV fork, "evolved into a near-complete rewrite" (TypeScript/Node pnpm monorepo, React UI). Adds Jellyfin/Emby/local-file sources. Same integration recipe: spoofed HDHR tuner + M3U + XMLTV. Scheduling: virtual timeline plus **time-slot and random-slot tools** and filler content (commercials, prerolls, bumpers).

What users praise ([comparison, June 2026](https://www.pistack.xyz/posts/2026-06-12-self-hosted-virtual-tv-channels-ersatztv-tunarr-dizquetv/)): "modern, polished" UI, drag-and-drop visual schedule editor that feels "more like using a DVR than configuring a server" — the acknowledged UX bar in this space, and the natural dizqueTV upgrade path.

What bites: standalone binaries **do not bundle ffmpeg** — "you must have your own build ready to go. We recommend the pre-built FFmpeg 7.1.1 binaries provided by ErsatzTV" ([install docs](https://tunarr.com/getting-started/installation/)); only Docker images ship ffmpeg. Long pre-1.0 beta; client-compatibility edge cases (streams fine in native Plex app, broken in third-party clients — [plezy #816](https://github.com/edde746/plezy/issues/816)). License **zlib** — borrowable.

### ErsatzTV
[Legacy C# repo](https://github.com/ErsatzTV/legacy) · [Rust rewrite](https://github.com/ErsatzTV/next) · [Docs](https://ersatztv.org/docs/). .NET app, zlib. HDHR emulation + XMLTV; sources from Plex/Jellyfin/Emby/local. The **power tool** of the space: deterministic playouts built ahead of time (honest guide data), broadest backend support, hardware transcoding (VAAPI/QSV/NVENC), and *five* schedule types — classic (ordered collections + playout modes flood/duration/multiple/one), time slots, block/template, YAML-defined, and scripted JavaScript ([scheduling docs](https://ersatztv.org/docs/scheduling/classic/)).

That power is also the complaint surface: a user must understand the chain *collections → schedules → schedule items → playouts* before anything plays, and features couple in surprising ways (enabling shuffle "disables fixed start times and flood mode"). Community materials (a marketplace "ErsatzTV Channel Scheduler" AI skill, third-party YAML playout repos) exist mainly to tame scheduling complexity.

**The most instructive 2026 signal:** the maintainer moved the C# monolith to `ErsatzTV/legacy` (still updated) and started `ErsatzTV/next` in **Rust, MIT-licensed, "VERY EARLY STAGE"**, with scope cut to "one thing well: dependable transcoding and streaming." Library management, metadata, and scheduling are **explicitly out of scope**; `next` consumes playout JSON documents and handles stream reliability. Five crates: transcoding logic, playout schema, single-channel stream generation, HTTP IPTV serving, demo playout generator. Read: after years of the full-featured monolith, its own author concluded (a) the ffmpeg/stream pipeline is the genuinely hard part, and (b) it should be a compiled, decoupled component. Both conclusions favor chanarr's Go-binary, small-scope design — and warn that mid-flight rewrites orphan users (the archived Windows/macOS wrappers, Vue UI, etc.).

## Cross-cutting conclusions

1. **HDHR emulation + XMLTV is settled science.** Every relevant project (dizqueTV, Tunarr, ErsatzTV, dispatcharr, Threadfin/xTeVe, even tvheadend-via-proxy) converges on it. Founding decision #2 is validated; three permissively-licensed reference implementations exist.
2. **Virtual timeline is the norm** (dizqueTV/Tunarr); ErsatzTV's deterministic pre-built playout is the stronger variant because the guide never lies. Decision #3 validated; consider deterministic playout computation (channel epoch + modular arithmetic over total playlist duration) rather than ad-hoc "what's playing now" state.
3. **FFmpeg handling is the recurring operational pain**: Tunarr binaries ship without it, ErsatzTV maintains its own ffmpeg builds, and ErsatzTV-next is entirely a bet that stream reliability is the hard core. Remux-first (decision #4) is right; the transcode-fallback path is where engineering risk concentrates. chanarr must locate/verify ffmpeg at startup and fail with a clear message (or document a pinned version).
4. **Nobody ships a true single static binary with embedded UI + DB.** dizqueTV's pkg binary and Tunarr's binaries are Node bundles that still need external ffmpeg; ErsatzTV needs .NET + ffmpeg; dispatcharr needs three services. Go + embedded React + SQLite is a real, unoccupied differentiator.
5. **Complexity in this space comes from two sources**: infrastructure (dispatcharr's Redis/Postgres/Celery) and conceptual model (ErsatzTV's scheduling chain, dispatcharr's IPTV jargon). chanarr must fight both: zero services beyond the binary, and "folder → channel" as the only concept a v1 user meets.

## Steal / avoid

**Steal**
- HDHR discovery/lineup endpoint behavior (`discover.json`, `lineup.json`, `lineup_status.json`) and XMLTV generation from dizqueTV/Tunarr — zlib, code borrowable with attribution.
- Virtual-timeline math from dizqueTV/Tunarr: channel start epoch + deterministic offset into the loop; makes tune-in mid-episode and honest guides fall out of one calculation.
- ErsatzTV's deterministic playout idea (build the schedule ahead, serve guide + stream from the same data) — without its collection/schedule/playout object model.
- Tunarr's UX bar: channel creation as a simple visual flow; guide preview inside the management UI.
- ErsatzTV-next's architecture insight: isolate the ffmpeg session / stream-pipeline as a sharply-bounded internal module (chanarr keeps it in-process, but the boundary should be as clean as their crate split).
- dizqueTV's on-demand vs continuous channel toggle as a *future* option (not v1).
- Single-image deployment as the marketing headline — dispatcharr's "AIO container" shows competitors can't say "no Redis, no Postgres, no Celery"; chanarr can.

**Avoid**
- dispatcharr's service sprawl (Redis/Postgres/Celery/Django) and any env-var plumbing on the happy path.
- IPTV-jargon-first onboarding (M3U accounts, Xtream Codes, EPG source mapping). chanarr's v1 vocabulary is: folder, channel, guide.
- ErsatzTV's five schedule types and collections→schedules→schedule-items→playouts concept chain — power users can be served later; v1 is folders→looping channel.
- Shipping binaries that silently require an external ffmpeg (Tunarr's trap). Verify at startup, guide the fix.
- Fragile channel/EPG state that breaks on library refresh (dispatcharr #493) — rescans must preserve channel identity.
- Plex-only assumptions welded in so deep that other outputs become impossible (dizqueTV's dead end) — but equally, multi-backend scope creep (Tunarr/ErsatzTV) before Plex works flawlessly.
- Perpetual pre-1.0 drift (Tunarr) and mid-life full rewrites (ErsatzTV) — keep v1 scope small enough to actually finish.
- Any code from tvheadend (GPL-3.0) or dispatcharr (AGPL-3.0); protocol observations only.
