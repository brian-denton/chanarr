# Research: Plex guide data via XMLTV

Type: research
Status: resolved

## Question

How does Plex consume XMLTV for an emulated tuner, and what does a rich guide require? Wiring XMLTV into Live TV & DVR setup, refresh cadence and how to trigger updates, channel numbering/mapping rules, episode titles/descriptions/artwork support, Plex Pass requirement and version constraints, and quirks (guide horizon length, schedule drift). How dizqueTV/Tunarr and ErsatzTV generate theirs.

## Answer

Full findings with sources and XMLTV snippets: [../assets/research-plex-guide-xmltv.md](../assets/research-plex-guide-xmltv.md)

- **Wiring**: during Live TV & DVR setup, after the tuner is confirmed, the user clicks "Have an XMLTV guide on your server? Click here to use that instead" and enters a URL (or PMS-local file path) — e.g. `http://chanarr:8000/api/xmltv.xml`. Plex Pass is required for Live TV & DVR (still true after the April 2025 pricing changes); no practical PMS minimum version.
- **Mapping**: Plex matches lineup.json `GuideNumber` against XMLTV channel id / display-names. Use the channel number as `<channel id>` and emit three display-names (`"1 Name"`, `"1"`, `"Name"`) for reliable auto-mapping; mapping can also be forced via `PUT /media/grabbers/devices/{deviceKey}/channelmap`.
- **Refresh**: Plex re-polls XMLTV only ~daily (unreliably), so chanarr must push: `GET /livetv/dvrs` then `POST /livetv/dvrs/{key}/reloadGuide` with an X-Plex-Token — exactly what dizqueTV/Tunarr do after every regeneration. This means chanarr needs an optional Plex URL + token setting.
- **Rendered fields**: title, sub-title (episode title), desc, date, category (English strings only), icon (programme + channel art, fetched by PMS — serve locally, keep URLs stable), series-id, episode-num (`xmltv_ns` preferred, emit `onscreen` too; season must be present), rating, `<previously-shown/>`. Times as `yyyyMMddHHmmss +0000`.
- **Horizon**: prior art publishes a rolling 12h window regenerated every 4h; make both configurable and also regenerate+push on any channel mutation.
- **Quirks**: guide/stream honesty requires deriving both from the same deterministic timeline function (chanarr's virtual-timeline design gives this for free); identical adjacent programmes render merged; filler should be split into capped titled placeholder blocks; Plex Windows desktop app currently fails on virtual tuners (Plex Web works).
