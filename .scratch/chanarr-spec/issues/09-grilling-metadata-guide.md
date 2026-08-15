# Grilling: metadata sources & guide richness

Type: grilling
Status: resolved
Blocked by: 03, 05

## Question

Where do episode titles, descriptions, and artwork come from — filename parsing only, local NFO files, or an online source (TVDB/TMDB, keys, rate limits)? How rich should the Plex guide be for v1, and what's the fallback when metadata is missing? Balance against the no-docs north star: online metadata must not add setup steps. Also decide here: whether v1 includes the optional Plex-server connection (URL + X-Plex-Token) that guide research showed is needed to push `reloadGuide` — Plex's own re-poll is only ~daily — and how that squares with zero-config onboarding.

## Answer

Grilled 2026-08-15, two rounds, all recommendations accepted.

- **Episode identity**: parse `SxxExx` out of filenames via regex (Sonarr/Plex convention) to populate real season/episode numbers in `episode-num` — pure local computation, no setup cost. Fallback when unmatched: Airing title = filename stripped of extension.
- **Online metadata (TVDB/TMDB): deferred entirely from v1.** Filename parsing satisfies Plex's rendering requirements without an account/key/network call; a clean additive v1.1 feature.
- **Channel logo**: auto-detect conventional local files (`poster.jpg`/`folder.jpg`) as a zero-effort default, with manual upload as an override in the UI.
- **Plex connection for guide push**: optional and prompted, not required at onboarding — a channel works and plays without it; the UI surfaces a one-click "connect Plex for instant guide updates" prompt afterward using Plex's PIN-based link flow (`plex.tv/link`), never a manually-copied token. Full rationale: [ADR 0002](../../../docs/adr/0002-optional-prompted-plex-connection.md).
- **Guide defaults**: 12-hour horizon, 4-hour regeneration (prior-art defaults), fixed in v1, not exposed as a setup-time decision.
- **Category/rating**: always emit `<category>Series</category>` (cheap, improves Plex's grid treatment); omit `<rating>` entirely (no data to report; an absent rating is honest, a fabricated one isn't).
