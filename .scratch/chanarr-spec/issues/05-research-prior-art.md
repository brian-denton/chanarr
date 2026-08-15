# Research: prior art survey

Type: research
Status: resolved

## Question

What should chanarr steal or avoid from prior art? Survey tvheadend and dispatcharr (user-named), plus dizqueTV, its successor Tunarr, and ErsatzTV: architecture, scheduling model, why dispatcharr's setup is complicated (the pain chanarr exists to fix), what makes ErsatzTV/Tunarr succeed or struggle, and licensing of each (GPL vs permissive) in case code or protocol details are borrowed.

## Answer

Full findings with source links: [assets/research-prior-art.md](../assets/research-prior-art.md)

- **HDHR emulation + XMLTV is settled science** — every project in the space (dizqueTV, Tunarr, ErsatzTV, dispatcharr, even tvheadend via tvhproxy) converges on it. Founding decision #2 validated, with three permissively-licensed reference implementations (dizqueTV/Tunarr/ErsatzTV, all zlib).
- **Why dispatcharr is complicated**: it solves a different problem (managing external IPTV subscriptions, not local media — no "point at a folder" path exists); onboarding is ~8 jargon-heavy steps (M3U accounts, Xtream Codes, EPG source mapping, manual EPG-to-channel matching that breaks on playlist refresh); and it runs Django + React + **Redis + PostgreSQL + Celery**, exposing infrastructure even in its all-in-one container. AGPL-3.0 — ideas only, no code.
- **dizqueTV** (Node, zlib, maintenance mode) pioneered the virtual timeline and HDHR-spoof recipe chanarr adopts; **Tunarr** (TS rewrite, zlib, active) sets the UX bar with its drag-and-drop guide editor but ships binaries without ffmpeg; **ErsatzTV** (C#, zlib) has the strongest scheduling (deterministic playouts = honest guides) but a heavy concept chain (collections → schedules → schedule items → playouts, five schedule types). In 2026 ErsatzTV's author moved the C# monolith to `legacy` and began a Rust/MIT rewrite scoped to *only* transcoding/streaming — confirming the ffmpeg stream pipeline is the hard core and small scope is right.
- **Open lane**: nobody ships a true single static binary with embedded UI + DB; every competitor needs Node/.NET runtimes, external ffmpeg, or multiple services. Go + embedded React + SQLite is a real differentiator; ffmpeg presence-check at startup is the one external dependency to handle gracefully.
- Steal: HDHR endpoints + XMLTV generation and timeline math from dizqueTV/Tunarr (zlib); deterministic playout idea from ErsatzTV; Tunarr's UX bar. Avoid: service sprawl, IPTV jargon, ErsatzTV's scheduling object model in v1, unbundled-ffmpeg surprises, GPL/AGPL code (tvheadend, dispatcharr).
