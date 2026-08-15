# Research: Plex guide data via XMLTV

Resolves issue 03. Researched 2026-08-15 against Plex support/forum sources and the XMLTV generation + Plex-notification code of dizqueTV, Tunarr, and ErsatzTV.

## 1. How XMLTV is wired into Plex DVR setup

- Live TV & DVR setup: **Settings → Live TV & DVR → Set Up Plex Tuner/DVR**. Plex discovers the emulated HDHomeRun via SSDP/discovery; manual entry of `http://ip:port` is the fallback.
- On the guide-selection step (after tuner confirm/channel scan), Plex shows a postal-code/Gracenote flow with a link: **"Have an XMLTV guide on your server? Click here to use that instead."** The user then picks a language and enters either a **local file path on the PMS machine or an HTTP(S) URL** to the XMLTV file, plus a display name for the guide. ([Plex support: Using XMLTV for guide data](https://support.plex.tv/articles/using-an-xmltv-guide/), [Tunarr Plex docs](https://tunarr.com/configure/clients/plex/), [ErsatzTV Plex docs](https://ersatztv.org/docs/clients/plex/))
- Plex provides no XMLTV data itself; the URL is fetched by the Plex server, so the chanarr host must be reachable **from the PMS machine** (matches founding constraint 6).
- Practical convention (all three prior-art apps): serve the guide from the same host/port as the tuner emulation, e.g. Tunarr `http://serverIP:8000/api/xmltv.xml`, dizqueTV `/api/xmltv.xml`, ErsatzTV `/iptv/xmltv.xml`.
- After the guide is loaded, Plex shows a **channel mapping screen**; with well-formed ids/display-names it reports "Your channels should automatically be mapped" and the user just confirms.

## 2. Channel-to-guide mapping rules

- The tuner side advertises channels in `lineup.json` with `GuideNumber` (channel number string) and `GuideName`. Plex maps each lineup channel to an XMLTV `<channel>` by matching the number against the XMLTV channel **id** and **display-name** entries.
- **dizqueTV** keeps it maximally simple: `<channel id="{number}">` with one `<display-name lang="en">` and programme `channel="{number}"` (src/xmltv.js). Channel number *is* the id. This auto-maps reliably.
- **ErsatzTV** uses a structured id `C{channelNumber}.{checksum}.{instanceId}.ersatztv.org` (ErsatzTV.Core/Iptv/ChannelIdentifier.cs) but then emits **three display-names** so Plex can match by number:
  ```xml
  <channel id="C1.123.abcd.ersatztv.org">
    <display-name>1 My Channel</display-name>
    <display-name>1</display-name>
    <display-name>My Channel</display-name>
    <icon src="{RequestBase}/iptv/logos/...jpg"/>
  </channel>
  ```
  (ErsatzTV/Resources/Templates/_channel.sbntxt). Tunarr does the same trio (`"{number} {name}"`, `"{number}"`, `"{name}"`) in server/src/services/XmlTvWriter.ts.
- **Recommendation for chanarr**: use the channel number as the XMLTV channel id AND emit the number-bearing display-name variants; keep `GuideNumber` in lineup.json identical. Mapping can also be forced programmatically (below).
- Channel numbers can be fractional (HDHomeRun "x.y" style); prior art sticks to integers by default.

## 3. Refresh cadence + programmatic refresh

- **Plex's own schedule**: PMS re-reads the XMLTV file/URL on the DVR's guide-refresh schedule — by default roughly **once a day (~2:00 AM)**; newer PMS builds expose "Guide Refresh Time"/interval in DVR settings. Community consensus: the built-in scheduler is unreliable and a daily pull is far too slow for a dynamic virtual-channel server. ([forum: Guide Refresh Time Setting](https://forums.plex.tv/t/guide-refresh-time-setting/753529), [forum: XMLTV Guide Not Updating](https://forums.plex.tv/t/xmltv-guide-not-updating-in-plex/707419))
- **Programmatic trigger — the critical mechanism.** All prior art pushes refreshes to Plex via its private HTTP API (authenticated with `X-Plex-Token`):
  1. `GET {pms}/livetv/dvrs` → list DVRs, each with a `key`, and nested `Device` entries with their own `key`.
  2. `POST {pms}/livetv/dvrs/{dvrKey}/reloadGuide` → forces PMS to re-fetch and re-ingest the XMLTV source. (dizqueTV src/plex.js `RefreshGuide`; Tunarr server/src/external/plex/PlexApiClient.ts:1167 `await this.doPost({ url: '/livetv/dvrs/${dvr.key}/reloadGuide' })`.)
  3. Optionally `PUT {pms}/media/grabbers/devices/{deviceKey}/channelmap` with query params `channelsEnabled=1,2,3` and per-channel `channelMapping[{n}]={n}` / `channelMappingByKey[{n}]={n}` → forces the lineup↔guide channel mapping without user interaction (dizqueTV src/plex.js:166-178). Tunarr dropped the channelmap call and only does reloadGuide.
- **Cadence in prior art**: dizqueTV regenerates its XMLTV every `refresh` hours (default **4h**, setting `xmltv-settings.refresh`; 0 disables) and fires `reloadGuide` after every regeneration and after any channel change (index.js `xmltvInterval` + `notifyPlex`, defaults in src/database-migration.js: `{cache: 12, refresh: 4}`). Tunarr: identical defaults (`refreshHours: 4` in types/src/schemas/settingsSchemas.ts), and its UpdateXmlTvTask calls `plex.refreshGuide(dvrs)` gated by a per-server `sendGuideUpdates` flag.
- **Implication for chanarr**: chanarr needs the user's Plex token + server URL to push guide updates — i.e., a small "Plex server connection" feature is required for a good guide experience even though the tuner emulation itself needs no Plex credentials. Without it, guide changes wait for Plex's (unreliable) daily pull.

## 4. XMLTV fields Plex actually renders

Community-established list (no official Plex schema doc exists): **title, sub-title, desc, date, category, language, icon, series-id, episode-num** ([forum: XMLTV code structure](https://forums.plex.tv/t/xmltv-code-structure/565163), [forum: XMLTV format to be used in Plex](https://forums.plex.tv/t/xmltv-format-to-be-used-in-plex/611664)).

- `<title>` — show/movie name (grid cell text).
- `<sub-title>` — episode title; shown in the pre-play/detail pane.
- `<desc>` — synopsis in detail pane.
- `<episode-num system="xmltv_ns">` — preferred; zero-based `"{s-1}.{e-1}.0/1"`. Plex needs a **season component present** for S/E display to work; `system="onscreen"` (`S05E19`) also parses but is considered less reliable. Episode identification requires episode-num or an original-air-date. All three apps emit both systems (dizqueTV src/xmltv.js, Tunarr XmlTvWriter.ts, ErsatzTV _episode.sbntxt: `S{{...'00'}}E{{...'00'}}` + `{{s-1}}.{{e-1}}.0/1`).
- `<icon src>` — programme artwork thumbnail in guide/detail; channel `<icon>` is the channel logo. Prior art serves artwork through its own HTTP cache (dizqueTV `{{host}}/cache/images/...`, Tunarr `{{host}}/api/programs/{id}/artwork/{type}`) because Plex fetches these URLs itself — they must be PMS-reachable and stable. ErsatzTV additionally emits `<image type="poster">`/`<image type="still">` (newer XMLTV element).
- `<category>` — genre badges/filters. Plex only recognizes a limited set of **English** category strings (e.g. "Movie"/"Movie / Drama" drives movie treatment, "News", "Sports"); foreign-language categories are ignored ([forum: XMLTV & Categories](https://forums.plex.tv/t/xmltv-categories/197790), [foreign categories not used](https://forums.plex.tv/t/xmltv-foreign-categories-not-used/507520)). ErsatzTV tags episodes `<category lang="en">Series</category>` plus show genres.
- `<date>` — production year (shown for movies).
- `<rating>` — content rating; dizqueTV emits `system="MPAA"`, ErsatzTV emits `system="VCHIP"` for `us:` ratings.
- `<previously-shown/>` — all three apps hardcode it on every programme (prevents Plex treating everything as "new"; harmless and recommended).
- Known rendering quirk: consecutive programmes with the **same title and same episode identity** get merged/displayed oddly in the grid ([forum thread](https://forums.plex.tv/t/plex-is-not-displaying-xmltv-guide-correctly-with-programs-that-have-the-same-title/770161)) — relevant to looping channels; distinct episode-nums avoid it.

### Datetime format

`start`/`stop` attributes: `yyyyMMddHHmmss ±zzzz`, e.g. `20260815143000 +0000`. dizqueTV emits UTC with a literal `" +0000"`; ErsatzTV formats `"yyyyMMddHHmmss zzz"` then strips the colon from the offset, configurable UTC-vs-local (`XmltvTimeZone`). Either works; UTC everywhere is simplest.

### Minimal known-good programme (composite of dizqueTV/ErsatzTV output)

```xml
<tv generator-info-name="chanarr">
  <channel id="1">
    <display-name>1 Cartoons</display-name>
    <display-name>1</display-name>
    <display-name>Cartoons</display-name>
    <icon src="http://chanarr:8000/logos/1.png"/>
  </channel>
  <programme start="20260815140000 +0000" stop="20260815143000 +0000" channel="1">
    <title lang="en">Some Show</title>
    <sub-title lang="en">The Episode Title</sub-title>
    <desc lang="en">Episode synopsis.</desc>
    <category lang="en">Series</category>
    <icon src="http://chanarr:8000/art/ep123.jpg"/>
    <episode-num system="onscreen">S02E05</episode-num>
    <episode-num system="xmltv_ns">1.4.0/1</episode-num>
    <rating system="MPAA"><value>TV-PG</value></rating>
    <previously-shown/>
  </programme>
</tv>
```

## 5. Plex Pass + version constraints

- **Live TV & DVR requires an active Plex Pass** on the server owner's account — unchanged by the April 2025 pricing overhaul (which raised Plex Pass to $6.99/mo, $69.99/yr, $249.99 lifetime and moved remote-streaming behind payment). ErsatzTV docs state it flatly: "A Plex Pass is required for ErsatzTV to work with Plex." ([plex.tv/tv](https://www.plex.tv/tv/), [2025 updates](https://www.plex.tv/blog/important-2025-plex-updates/), [ErsatzTV docs](https://ersatztv.org/docs/clients/plex/))
- No meaningful PMS minimum-version constraint in practice: DVR + XMLTV has existed for years and every remotely current PMS (1.2x+) supports it. Newer builds add the guide-refresh-time UI. One current client quirk (Tunarr docs): the **Plex Windows desktop app** errors ("This Live TV session has ended") on these virtual tuners; Plex Web works.
- Only MPEG-TS streaming is directly consumed by Plex's tuner path (ErsatzTV docs) — consistent with chanarr's remux-to-TS decision.

## 6. Guide horizon

- Prior-art defaults: **12 hours of programming, regenerated every 4 hours** (dizqueTV `cache: 12, refresh: 4`; Tunarr `programmingHours: 12, refreshHours: 4`). This is the proven sweet spot: the virtual timeline is deterministic, so publishing further ahead costs XML size and DB work for no benefit as long as regeneration+push keeps the window rolling.
- If the published window is exhausted (regen/push fails), Plex shows empty/"unknown airing" cells beyond the data ([forum: EPG Unknown Airing](https://forums.plex.tv/t/epg-unknown-airing-live-3-hours/840137)). Regeneration must therefore run at a period comfortably shorter than the horizon (4h vs 12h gives 3x margin).
- Recommendation: make both configurable, default horizon 12–24h, refresh 4h, plus regenerate+push immediately on any channel/schedule mutation.

## 7. Quirks / gotchas

1. **Push, don't wait**: Plex's own XMLTV re-poll is ~daily and historically flaky; every serious implementation pushes `reloadGuide` after each regeneration. Chanarr must store a Plex URL + token to do this (optional but strongly recommended feature).
2. **Guide vs stream drift**: the guide is only honest if the streamer and the guide generator derive from the *same deterministic timeline function* (channel epoch + program durations). dizqueTV's tv-guide-service carries explicit drift-correction code trimming program start/duration to keep boundaries aligned; chanarr's virtual-timeline design avoids drift by construction — compute "what plays at time T" from one source of truth for both the TS streamer and the XMLTV writer. Clock skew between chanarr host and PMS shows up as guide offset; emitting UTC offsets and using NTP-synced hosts is the mitigation.
3. **Flex/offline filler**: dizqueTV splits filler ("flex") into capped-length dummy programmes (`TVGUIDE_MAXIMUM_FLEX_DURATION`) so the grid doesn't show one enormous empty block. If chanarr v1 has any dead air, give it a titled placeholder programme.
4. **Identical adjacent programmes** can render merged in the grid — ensure looping schedules emit correct distinct episode-nums.
5. **Artwork URLs are fetched by PMS**, sometimes repeatedly — serve/cache them locally (dizqueTV added an image cache for exactly this) and keep them stable across regenerations.
6. **Categories are English-only** for Plex's badge logic; emit "Series"/"Movie" plus real genres.
7. Channel lineup changes (add/remove/renumber) need both `reloadGuide` and (to skip manual remapping) the `channelmap` PUT; otherwise the user must re-run the mapping step in Plex settings.

## Sources

- Plex support: https://support.plex.tv/articles/using-an-xmltv-guide/ , https://support.plex.tv/articles/225877347-live-tv-dvr/
- dizqueTV: https://github.com/vexorian/dizquetv — src/xmltv.js, src/plex.js (reloadGuide/channelmap), src/services/tv-guide-service.js, src/database-migration.js (defaults), index.js (xmltvInterval/notifyPlex)
- Tunarr: https://github.com/chrisbenincasa/tunarr — server/src/services/XmlTvWriter.ts, server/src/tasks/UpdateXmlTvTask.ts, server/src/external/plex/PlexApiClient.ts, types/src/schemas/settingsSchemas.ts; docs https://tunarr.com/configure/clients/plex/
- ErsatzTV: https://github.com/ErsatzTV/ErsatzTV — ErsatzTV.Core/Iptv/ChannelIdentifier.cs, ErsatzTV.Application/Channels/Commands/RefreshChannelDataHandler.cs, ErsatzTV/Resources/Templates/_channel.sbntxt and _episode.sbntxt; docs https://ersatztv.org/docs/clients/plex/
- Plex forums: XMLTV fields https://forums.plex.tv/t/xmltv-code-structure/565163 , episode-num https://forums.plex.tv/t/xmltv-format-to-be-used-in-plex/611664 , categories https://forums.plex.tv/t/xmltv-categories/197790 and https://forums.plex.tv/t/xmltv-foreign-categories-not-used/507520 , refresh https://forums.plex.tv/t/guide-refresh-time-setting/753529 and https://forums.plex.tv/t/xmltv-guide-not-updating-in-plex/707419 , same-title quirk https://forums.plex.tv/t/plex-is-not-displaying-xmltv-guide-correctly-with-programs-that-have-the-same-title/770161
- Plex Pass/pricing: https://www.plex.tv/tv/ , https://www.plex.tv/blog/important-2025-plex-updates/
