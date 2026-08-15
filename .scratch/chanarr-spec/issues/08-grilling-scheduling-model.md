# Grilling: virtual-timeline scheduling domain model

Type: grilling
Status: resolved
Blocked by: 04

## Question

Pin down the scheduling domain model: is the timeline a pure deterministic function of (channel start anchor + ordered playlist + durations), or a persisted schedule? What happens when files are added/removed/shuffled mid-loop — does the timeline shift, regenerate from now, or preserve history? Per-channel config surface (start anchor, shuffle seed, gap handling for duration drift). Produces the ubiquitous language for channel/schedule/program entities via /domain-modeling.

## Answer

Grilled 2026-08-15, three rounds, all recommendations accepted. Full vocabulary in [CONTEXT.md](../../../CONTEXT.md); the core architectural choice is [ADR 0001](../../../docs/adr/0001-epoch-based-pure-timeline-function.md).

- **Model**: a single pure function `ProgramAt(channel, t) → Airing`, called by both the streamer (point query at tune-in) and the XMLTV writer (range query = repeated point queries walking to each Airing's end). No persisted/materialized schedule — this is the mechanism that structurally prevents stream/guide drift.
- **Epoch versioning**: any playlist-membership or ordering change (add/remove/reorder/shuffle-toggle/seed-change) stamps a new, immutable **Epoch** uniformly — no special-casing "safe" edits. `epochStart = creation time`, no user-settable anchor in v1.
- **Shuffle**: seeded, fixed for the epoch's whole cycle (not reshuffled per loop) — required so the guide can honestly publish a not-yet-started cycle's order.
- **Duration drift/failures**: filler/error-screen padded to the expected duration, keeping all future Airing boundaries stable regardless of runtime playback failures.
- **Duration sourcing**: ffprobed once at epoch creation and cached in SQLite — never re-probed in the `ProgramAt` hot path.
- **Channel mapping**: one Channel = one folder, scanned recursively; in-order mode uses lexical path/filename sort (no season/episode parsing needed for ordering — that's metadata territory, ticket 09).

Carried into fog: multi-folder-per-channel composition (explicitly deferred, not v1) and library-scanning/watch-folder cadence (already fog; now understood to mean "detecting a folder change and stamping a new epoch").
