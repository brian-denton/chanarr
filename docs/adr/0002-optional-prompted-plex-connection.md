# Plex connection (for guide push) is optional and prompted, not required at onboarding

Status: accepted

Plex's own XMLTV re-poll is roughly daily and unreliable, so a fresh or edited guide only appears promptly if chanarr pushes `reloadGuide` to PMS via an authenticated token — every prior-art project (dizqueTV, Tunarr) does this. We considered making that connection a required onboarding step, since it's genuinely needed for a good guide experience.

We rejected requiring it: the "folder → channel playing in Plex in under 10 minutes, no docs" north star is chanarr's core differentiator, and a channel works — Plex still tunes and plays it — without guide push; only the guide's freshness degrades. Onboarding completes with zero Plex credentials. Afterward, the UI surfaces a one-click "connect Plex for instant guide updates" prompt using Plex's PIN-based link flow (`plex.tv/link`) — no manually-copied `X-Plex-Token` — so the capability is discoverable without ever blocking the first channel.

Considered and rejected: requiring it up front (works against the north star for a benefit — guide freshness — that isn't visible until well after setup); shipping it silently optional with no prompt (users would never discover why their guide updates slowly).
