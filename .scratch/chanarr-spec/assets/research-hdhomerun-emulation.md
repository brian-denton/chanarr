# HDHomeRun emulation for Plex — protocol research

Researched 2026-08-15 against primary sources: xTeVe/Threadfin source, dizqueTV/Tunarr source, tvhProxy, telly wiki, Tvheadend's HDHR-emulation PR, and Silicondust's official HTTP API docs.

## 1. Required HTTP endpoints

Plex Live TV & DVR needs exactly three GET endpoints. Everything else is optional.

| Endpoint | Required | Purpose |
|---|---|---|
| `GET /discover.json` | **yes** | Device identity; fetched at setup and polled afterward |
| `GET /lineup.json` | **yes** | Channel list with stream URLs |
| `GET /lineup_status.json` | **yes** | Scan state (static stub is fine) |
| `GET /device.xml` | recommended | UPnP descriptor; used by SSDP auto-discovery path |
| `POST /lineup.post` | optional | Real devices accept `?scan=start`; emulators omit it (xTeVe/dizqueTV/Tunarr don't implement it) and Plex works fine |
| SSDP advertisement | optional | See §4 |

All three JSON endpoints should return `Content-Type: application/json` (xTeVe sets it explicitly; [webserver.go](https://github.com/xteve-project/xTeVe/blob/master/src/webserver.go) lines 75–96). xTeVe also aliases `/device.xml` at `/capability`.

## 2. JSON shapes

### discover.json

Real HDHomeRun ([Silicondust HTTP API](https://info.hdhomerun.com/info/http_api)):

```json
{
  "FriendlyName": "HDHomeRun FLEX 4K",
  "ModelNumber": "HDFX-4K",
  "FirmwareName": "hdhomerun_dvr_atsc3",
  "FirmwareVersion": "20260326",
  "DeviceID": "10AFFFFF",
  "DeviceAuth": "sfqnCjyZhgyqkBr2oFvclmWF",
  "BaseURL": "http://hdhr-10afffff.local",
  "LineupURL": "http://hdhr-10afffff.local/lineup.json",
  "TunerCount": 4
}
```

What emulators actually ship (all confirmed working with Plex):

dizqueTV ([src/hdhr.js](https://github.com/vexorian/dizquetv/blob/main/src/hdhr.js)) — Tunarr ([server/src/services/HDHRService.ts](https://github.com/chrisbenincasa/tunarr/blob/main/server/src/services/HDHRService.ts)) is byte-for-byte the same shape with "Tunarr" substituted:

```json
{
  "FriendlyName": "dizqueTV",
  "Manufacturer": "dizqueTV - Silicondust",
  "ManufacturerURL": "https://github.com/vexorian/dizquetv",
  "ModelNumber": "HDTC-2US",
  "FirmwareName": "hdhomeruntc_atsc",
  "TunerCount": 1,
  "FirmwareVersion": "20170930",
  "DeviceID": "dizqueTV",
  "DeviceAuth": "",
  "BaseURL": "http://host:8000",
  "LineupURL": "http://host:8000/lineup.json"
}
```

xTeVe/Threadfin ([hdhr.go](https://github.com/xteve-project/xTeVe/blob/master/src/hdhr.go)): `Manufacturer: "Golang"`, `ModelNumber: <app version>`, `FirmwareName: "bin_<version>"`, `DeviceAuth: <app name>`, `DeviceID: <settings UUID>`. tvhProxy: `Manufacturer: "Silicondust"`, `ModelNumber: "HDTC-2US"`, `FirmwareName: "hdhomeruntc_atsc"`, `DeviceID: "12345678"`, `DeviceAuth: "test1234"`.

Field rules distilled:

- **DeviceID** — real devices use 8 hex digits with a checksum nibble (validated by `libhdhomerun`'s `hdhomerun_discover_validate_device_id`), but **Plex does not validate format or checksum**: dizqueTV ships the literal string `"dizqueTV"`, Tunarr `"Tunarr"`, xTeVe a UUID-ish string. What matters: it must be **unique** on the network (two emulators with the same ID collide in Plex) and **stable** — Plex keys the configured DVR to it (`device://tv.plex.grabbers.hdhomerun/<DeviceID>`); changing it orphans the DVR ("device died") and requires re-setup. xTeVe deliberately exploits this: it appends `:<tunerCount>` to the DeviceID when tuner count > 1 ([system.go `setDeviceID()`](https://github.com/xteve-project/xTeVe/blob/master/src/system.go)) so that changing the tuner count forces Plex to see a "new" device — because Plex caches TunerCount from setup time and won't notice an in-place change.
- **DeviceAuth** — used by real devices to authenticate against Silicondust's cloud guide. Plex passes it to the HDHomeRun EPG grabber, which chanarr won't use (XMLTV instead). Any string works, including empty (dizqueTV/Tunarr ship `""`).
- **ModelNumber / FirmwareName** — not validated. Convention is `"HDTC-2US"` + `"hdhomeruntc_atsc"` (the HDHomeRun EXTEND, a transcode-capable model that emits H.264 in TS — the safest identity for a server that serves H.264 rather than broadcast MPEG-2). Follow the convention; don't invent.
- **BaseURL / LineupURL** — absolute URLs, must be reachable **from the Plex server**. Best practice (dizqueTV/Tunarr/tvhProxy): build them from the incoming request's `Host` header so they're always right for whoever's asking. Never `localhost` (antennas README: "Plex doesn't like localhost").
- **FriendlyName** — free text, shown in the Plex UI.

### lineup.json

Array of channel objects; three fields each:

```json
[
  { "GuideNumber": "1", "GuideName": "Simpsons 24/7", "URL": "http://host:8000/stream/channels/abc123.ts" }
]
```

- `GuideNumber` is a **string** (real devices use `"5.1"` style for ATSC; plain integers-as-strings are fine and are what all emulators use). This is the number Plex matches XMLTV channels against.
- `URL` is used by Plex **verbatim** — the path is entirely yours; there is no requirement to mimic the real device's `/auto/v{n}` scheme. dizqueTV uses `/video?channel=N`; Tunarr moved to `/stream/channels/{uuid}.ts` with an explicit code comment that query parameters were avoided because "Plex doesn't handle them well" ([hdhrApi.ts](https://github.com/chrisbenincasa/tunarr/blob/main/server/src/api/hdhrApi.ts)). **Use a clean path, no query string, optional `.ts` suffix.**
- Real devices also support `Tags` (`"favorite"`, `"drm"`) — Plex hides DRM-tagged channels; omit the field.
- dizqueTV/Tunarr return a placeholder channel when the lineup is empty (Plex setup fails confusingly on an empty array).

### lineup_status.json

Static stub, two variants in the wild, both fine:

```json
{ "ScanInProgress": 0, "ScanPossible": 1, "Source": "Cable", "SourceList": ["Cable"] }
```

(dizqueTV/Tunarr use `ScanPossible: 1`; xTeVe/Threadfin use `ScanPossible: 0`. Plex's "scan channels" step just re-fetches lineup.json either way.)

### device.xml

UPnP root-device descriptor pointing at BaseURL; dizqueTV/Tunarr template:

```xml
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <URLBase>{BaseURL}</URLBase>
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>
    <friendlyName>chanarr</friendlyName>
    <manufacturer>Silicondust</manufacturer>
    <modelName>HDTC-2US</modelName>
    <modelNumber>HDTC-2US</modelNumber>
    <serialNumber/>
    <UDN>uuid:{stable-uuid}</UDN>
  </device>
</root>
```

## 3. Discovery: SSDP vs manual IP

- Real HDHomeRun discovery is a **proprietary Silicondust UDP broadcast protocol on port 65001**, not SSDP ([Tvheadend #5236](https://old.tvheadend.org/issues/5236)). No emulator studied implements it.
- Some emulators additionally advertise via **SSDP** (`upnp:rootdevice` + `urn:schemas-upnp-org:device:MediaServer:1`, location → `/device.xml`): dizqueTV and Tunarr via `node-ssdp`, xTeVe via `koron/go-ssdp` (ST `upnp:rootdevice`, USN `uuid:{DeviceID}::upnp:rootdevice`, maxAge 1800, re-alive every 5 min) — and xTeVe makes it a **settings toggle, off-able**. This sometimes lets Plex auto-populate the tuner tile, but it's unreliable (breaks under Docker bridge networking — antennas README notes discovery from Plex doesn't work in containers) and Tvheadend's implementation skipped it entirely ([tvheadend PR #1348](https://github.com/tvheadend/tvheadend/pull/1348): "auto-discovery … does not work since clients seem to use different mechanisms").
- **Manual IP entry always works and is the path to design for.** In Plex's DVR setup: "Don't see your HDHomeRun device? Enter its network address manually" → user types `ip:port` → Plex GETs `http://ip:port/discover.json`. Known UI quirk ([telly wiki](https://github.com/tellytv/telly/wiki/Adding-Telly-to-Plex)): the device tile often does **not** appear after manual entry, but the Continue button becomes clickable — proceed anyway.
- Verdict for chanarr: ship manual-IP as the documented flow; SSDP is a cheap optional add (one goroutine, `koron/go-ssdp`) that helps LAN/host-network installs, but must not be load-bearing.

## 4. TunerCount semantics

- Plex reads `TunerCount` at setup and **enforces the concurrency limit client-side**: it counts its own active streams/recordings against the number and refuses to start more ([tvheadend PR #1348]: "Plex counts how many are in use so it will stop when it reaches that number").
- Plex caches the value from setup; raising it later in the emulator is not picked up until the tuner is re-added (this is why xTeVe bakes tuner count into the DeviceID).
- Server-side enforcement is optional but correct: a real device answers an over-limit tune with **HTTP 503 + header `X-HDHomeRun-Error: 805` ("All Tuners In Use")**; other documented codes: 801 Unknown Channel, 810 DVR Full, 811 Content Protection Required ([Silicondust HTTP API](https://info.hdhomerun.com/info/http_api)). Plex surfaces this as "could not tune channel".
- Emulator defaults: xTeVe 1, tvhProxy 1, Tvheadend 6, dizqueTV/Tunarr configurable. For chanarr each concurrent stream costs an ffmpeg process; advertise a configurable count (default 2–4 is sensible) and enforce it with 503+805 so behavior is defined when Plex's client-side count drifts.

## 5. Stream serving

- Plex GETs the lineup URL and expects an **MPEG-TS byte stream** on the response body, unbounded length, chunked/streamed (no Content-Length). Real devices label it per the HTTP API docs; use `Content-Type: video/mp2t`.
- Real-device URL grammar (`/auto/v{ch}`, `/tuner{n}/v{ch}`, `?duration=`, `?transcode={heavy|mobile|internet540|...}`) is **not needed** — Plex never constructs URLs itself for a network tuner, it only replays lineup.json URLs.
- Start pushing TS bytes **fast**. Plex times out slow tuners ("Could not tune channel"); the standard trick (dizqueTV et al.) is to begin emitting transport stream immediately (padding/PAT/PMT from ffmpeg) rather than buffering seconds of content first.
- Expect Plex to open and quickly abort probe connections (during setup scan and channel preview). Treat every disconnect as normal teardown; kill the ffmpeg child promptly or tuners leak.
- No auth is possible on any of these endpoints — Plex has no way to send credentials to a network tuner. Access control is network-level only (the Tvheadend PR review reached the same conclusion).

## 6. Gotchas checklist

1. **Never localhost** in BaseURL/LineupURL; derive from the request Host header.
2. **Stable DeviceID** — persist it; regenerating orphans the Plex DVR ("device `device://tv.plex.grabbers.hdhomerun/<id>` died").
3. **Plex polls discover.json** after setup; if chanarr is down long enough Plex marks the device dead and Live TV greys out until chanarr returns (no re-setup needed if DeviceID unchanged).
4. **No query strings in stream URLs** (Tunarr's hard-won lesson).
5. **TunerCount changes need tuner re-add** in Plex (or the xTeVe DeviceID-suffix trick).
6. **Empty lineup breaks setup** — return a placeholder channel if no channels are configured.
7. **Docker bridge kills SSDP**; document host networking or manual IP.
8. **GuideNumber must match XMLTV channel numbering** or the user faces manual channel-mapping in Plex; emitting our own XMLTV with matching numbers gets auto-matching (telly wiki observation).
9. **Setup UI quirk**: manually entered tuner may show no tile — Continue still works.

## Sources

- Silicondust official HTTP API: https://info.hdhomerun.com/info/http_api
- xTeVe: https://github.com/xteve-project/xTeVe (src/hdhr.go, src/ssdp.go, src/system.go, src/webserver.go)
- Threadfin (xTeVe fork, same protocol surface): https://github.com/Threadfin/Threadfin (src/hdhr.go)
- dizqueTV: https://github.com/vexorian/dizquetv/blob/main/src/hdhr.js
- Tunarr: https://github.com/chrisbenincasa/tunarr (server/src/api/hdhrApi.ts, server/src/services/HDHRService.ts)
- antennas (Tvheadend→HDHR proxy): https://github.com/TheJF/antennas
- tvhProxy (minimal reference): https://github.com/PREngineer/TVHProxy
- telly Plex setup wiki: https://github.com/tellytv/telly/wiki/Adding-Telly-to-Plex
- Tvheadend HDHR emulation PR: https://github.com/tvheadend/tvheadend/pull/1348 and https://old.tvheadend.org/issues/5236
- HDHomeRun protocol notes: https://github.com/waypar/hdhomerun-protocol-docs
