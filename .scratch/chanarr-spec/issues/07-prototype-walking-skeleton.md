# Prototype: walking skeleton — Plex tunes one looping channel

Type: prototype
Status: resolved
Blocked by: 02, 04

## Question

De-risk the core assumption: a minimal Go program that answers HDHomeRun discovery, exposes one channel, and serves a looping ffmpeg-remuxed MPEG-TS stream — does real Plex detect it, show it, and play it smoothly across an episode boundary? Throwaway code; the answer is what works, what breaks, and what the spec must therefore mandate.

## Answer

**Confirmed working** (user-tested 2026-08-15 against their real Plex): the minimal chain — three JSON endpoints + XMLTV + per-request `ffmpeg -re -stream_loop -1 -f concat -c copy -f mpegts` — was detected, set up, and played by Plex. Prototype (Go, ~130 lines) at [assets/walking-skeleton/](../assets/walking-skeleton/), run instructions in its README.

What the spec can now mandate as proven:
- Host-header-derived BaseURL/lineup/stream/guide URLs work end-to-end.
- Manual IP:port tuner entry works (tuner ran on a different box than PMS).
- Per-tuner-request ffmpeg spawn with kill-on-disconnect is a sufficient lifecycle for v1.
- Copy-mode concat of identically-encoded files produces a single consistent TS program (PIDs stable across loop); self-captured 70s spanned episode boundary + loop point with only transient splice warnings (one duplicate-DTS, one MB decode error).

Caveats carried forward:
- User verdict was "appears to work" — boundary-glitch severity under real heterogeneous media is untested; identical encode params made this the easy case. The remux-vs-normalize gate (scheduling/pipeline spec) still decides the hard case.
- PMS version still uncaptured (owed since ticket 06).
