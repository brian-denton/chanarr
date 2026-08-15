# Founding decisions

Type: grilling
Status: resolved

## Question

What is chanarr, what is the destination of this map, and what are its founding constraints?

## Answer

Grilled in the charting session (2026-08-15); all recommendations accepted:

1. **Destination**: complete v1 spec + locked architecture decisions, handed off to implementation sessions. The map plans; it does not build.
2. **Plex integration**: emulate an HDHomeRun network tuner (discovery + lineup + MPEG-TS streams) plus an XMLTV guide — the proven dizqueTV/Threadfin/xTeVe route. Plex-first; generic M3U output is a cheap add that must not shape v1.
3. **Channel behavior**: virtual timeline — each channel runs a continuous 24/7 schedule; tuning in joins mid-episode like real TV, making guide data honest.
4. **Transcoding**: remux-to-MPEG-TS by default, automatic transcode fallback only when codecs/parameters demand it.
5. **v1 scope**: folders → channels (name/number/logo) → looping virtual timeline → Plex tuner + guide → React management UI; per-channel shuffle/in-order toggle included. Out: auth, bumpers/interstitials, hardware transcoding, non-Plex clients, smart scheduling.
6. **Identity & deployment**: name **chanarr**; single static Go binary with embedded React build and SQLite; Docker as a thin wrapper. Must run anywhere network-reachable from the Plex server.
7. **User-friendliness bar**: "folder → channel playing in Plex in under 10 minutes, without reading docs" is a testable v1 requirement.
