# Research: continuous MPEG-TS stream pipeline (dizqueTV / Tunarr / ErsatzTV)

Researched 2026-08-15 against project source and docs. Sources:

- dizqueTV (vexorian fork): [src/ffmpeg.js](https://github.com/vexorian/dizquetv/blob/main/src/ffmpeg.js), [src/video.js](https://github.com/vexorian/dizquetv/blob/main/src/video.js)
- Tunarr: [server/src/stream/](https://github.com/chrisbenincasa/tunarr/tree/main/server/src/stream) (`ConcatStream.ts`, `ProgramStream.ts`, `SessionManager.ts`, `ConnectionTracker.ts`), [server/src/ffmpeg/](https://github.com/chrisbenincasa/tunarr/tree/main/server/src/ffmpeg) (`FfmpegStreamFactory.ts`, `GetLastPtsDuration.ts`, `builder/pipeline/BasePipelineBuilder.ts`); docs: [Transcoding](https://tunarr.com/configure/channels/transcoding/), [FAQ](https://tunarr.com/misc/faq/)
- ErsatzTV: [ErsatzTV.Core/FFmpeg/FFmpegLibraryProcessService.cs](https://github.com/ErsatzTV/ErsatzTV/blob/main/ErsatzTV.Core/FFmpeg/FFmpegLibraryProcessService.cs), [ErsatzTV.Application/Streaming/HlsSessionWorker.cs](https://github.com/ErsatzTV/ErsatzTV/blob/main/ErsatzTV.Application/Streaming/HlsSessionWorker.cs), [ErsatzTV.FFmpeg/Pipeline/PipelineBuilderBase.cs](https://github.com/ErsatzTV/ErsatzTV/blob/main/ErsatzTV.FFmpeg/Pipeline/PipelineBuilderBase.cs); docs: [Channels / streaming modes](https://ersatztv.org/docs/channels/)
- ffmpeg docs: [ffmpeg.html (-ss semantics)](https://ffmpeg.org/ffmpeg.html), [ffmpeg-formats.html (concat demuxer)](https://ffmpeg.org/ffmpeg-formats.html)

---

## 1. How one continuous MPEG-TS is produced across heterogeneous files

Nobody runs a single ffmpeg over the whole playlist. All three projects use **one ffmpeg process per program item**, plus a joining mechanism:

### dizqueTV: two-tier ffmpeg (concat-demuxer tier + per-program tier)

- Client hits `/video` → server spawns **one long-lived "concat" ffmpeg per client** whose input is an ffconcat playlist served by the app itself (`/playlist?channel=N`). The playlist is `ffconcat version 1.0` with ~100 `file 'http://localhost:PORT/stream?channel=N&...'` lines.
- Each `/stream` URL, when the concat demuxer opens it, spawns a **per-program ffmpeg** that seeks into the current file, normalizes, and emits MPEG-TS on stdout.
- Concat tier (from `ffmpeg.js` `spawnConcat`), representative:

```
ffmpeg -threads 1 -fflags +genpts+discardcorrupt+igndts \
  -f concat -safe 0 -protocol_whitelist file,http,tcp,https,tcp,tls \
  -probesize 32 -i http://localhost:PORT/playlist?channel=N \
  -map 0:v -map 0:a -c copy \
  -muxdelay <concatMuxDelay> -muxpreload <concatMuxDelay> \
  -f mpegts pipe:1
```

- Per-program tier (from `spawn`), representative:

```
ffmpeg -threads N -fflags +genpts+discardcorrupt+igndts \
  [-re] [-ss <offsetIntoFile>] -i <file> \
  -map 0:v -map 0:a \
  -c:v <copy | encoder> -c:a <copy | encoder> \
  [-filter_complex "scale=W:H:flags=<algo>[scaled];pad=W:H:(ow-iw)/2:(oh-ih)/2[blackpadded]" ...] \
  [-shortest] [apad=whole_dur=<duration>ms for audio align] \
  -muxdelay 0 -muxpreload 0 -map_metadata -1 \
  [-t <duration>] -f mpegts pipe:1
```

`-ss` is placed **before `-i`** (input seek). `-re` throttles to realtime. `-t` bounds each item at the program boundary.

### Tunarr (TS rewrite of dizqueTV): same two-tier for MPEG-TS mode; HLS session is default

- Stream modes per channel (docs): **HLS (recommended, one ffmpeg per program feeding an HLS session)**, HLS "alt" (two processes: per-program raw normalize + concat/encode; the per-program leg *requires software encoding*), **HLS Direct / Direct v2** (no normalization, remux only), **MPEG-TS** ("closest to the dizqueTV experience… a lot of potential issues").
- `BasePipelineBuilder.concat()` emits (option objects → flags): `ConcatInputFormatOption` (`-f concat`), `InfiniteLoopInputOption` (`-stream_loop -1`), `ReadrateInputOption(caps, 0)` (`-readrate 1.0` with no initial burst), `ConcatHttpReconnectOptions` for HTTP sources, `CopyAllEncoder` (`-c copy`), `NoDemuxDecodeDelayOutputOption` (`-muxdelay 0`), `ClosedGopOutputOption`, `MpegTsOutputFormatOption` + `PipeProtocolOutputOption` (`-f mpegts pipe:1`).
- `hlsWrap()` (MPEG-TS served by remuxing its own HLS output): `-readrate` + 15M realtime buffer, `-map 0`, `-c copy`, `-f mpegts pipe:1`.
- Sessions are shared: `SessionManager`/`ConnectionTracker` keep one session per channel and count connected clients rather than one pipeline per client; `ProgramStream` = "lineup item + transcode session", killed/replaced per item, with automatic swap to an error-screen stream on failure.

### ErsatzTV: per-item ffmpeg into a channel HLS session; MPEG-TS mode is a copy-wrapper

- Primary mode **HLS Segmenter**: a per-channel `HlsSessionWorker` runs one ffmpeg per playout item, writing `.ts` segments + a live m3u8. Docs: HLS Segmenter "offers the best performance at program boundaries".
- **MPEG-TS mode** (what an HDHomeRun-emulating tuner uses) is "a light wrapper over the HLS Segmenter": `FFmpegLibraryProcessService.WrapSegmenter` builds

```
ffmpeg -threads 1 -nostdin -hide_banner -loglevel error \
  -i http://localhost:{port}/iptv/channel/{n}.m3u8?mode=segmenter \
  -map 0 -c copy -f mpegts pipe:1
```

- **MPEG-TS (Legacy)** mode is the dizqueTV-style direct concat (`/ffmpeg/concat/{n}?mode=ts-legacy` fed through `pipelineBuilder.Concat(...)` with `-f concat … -c copy -f mpegts pipe:1`); docs: "some clients will have issues at program boundaries", slated for removal.
- **HLS Direct** is the remux mode: `-c copy`, no watermarks, "can perform better on low power systems", "some clients will have issues at program boundaries".

**Takeaway:** the industry pattern is *ffmpeg-per-program + a continuity layer* (concat demuxer, or an HLS playlist with discontinuity tags, optionally re-wrapped to TS with `-c copy`). No one keeps a single ffmpeg alive across heterogeneous inputs.

---

## 2. Mid-file join (virtual-timeline tune-in)

- All three compute wall-clock offset into the currently-airing item and pass it as an **input-side `-ss`** (dizqueTV `spawn` puts `-ss startTime` before `-i`; Tunarr `StreamSeekInputOption(state.start)` on video and audio inputs; ErsatzTV `playbackSettings.StreamSeek` from `inPoint` → `StreamSeekInputOption`).
- ffmpeg semantics ([docs](https://ffmpeg.org/ffmpeg.html)): input `-ss` "seeks to the closest seek point before position"; when transcoding, the gap up to the requested position is decoded and discarded → **frame-accurate join**. With `-c copy` (or `-noaccurate_seek`) the pre-keyframe packets are preserved → join snaps to the prior keyframe and can emit undecodable leading frames (players show frozen/black/garbled video until the next IDR). This is precisely why the copy-based modes (HLS Direct, ts-legacy) have "issues at program boundaries" and why a remux-first design must accept keyframe-granularity tune-in (typically ≤ 2–10 s off, GOP-dependent).
- ErsatzTV tracks position by wall clock inside the session: `now = wasSeekAndWorkAhead ? DateTimeOffset.Now : _transcodedUntil` — only the *first* item after tune-in/seek uses a mid-file `-ss`; subsequent items start at 0 and are transcoded slightly ahead of realtime.

---

## 3. PTS/DTS continuity across file boundaries

Three distinct mechanisms observed:

1. **concat demuxer rewrite** (dizqueTV, ErsatzTV ts-legacy): ffmpeg's concat demuxer guarantees "the timestamps in the files are adjusted so that the first file starts at 0 and each next file starts where the previous one finishes" ([formats docs](https://ffmpeg.org/ffmpeg-formats.html)). dizqueTV belt-and-suspenders this with `-fflags +genpts+discardcorrupt+igndts` on **both** tiers. Requirement: "All files must have the same streams (same codecs, same time base, etc.)" — hence the per-program normalization tier feeding it uniform MPEG-TS.
2. **Explicit `-output_ts_offset` per item** (Tunarr, ErsatzTV): the next item's ffmpeg is started with the previous output's last PTS as a muxer offset. Tunarr `GetLastPtsDuration.ts` runs `ffprobe … -show_entries packet=pts,duration -of compact=p=0:nk=1`, takes the last line, and next session gets `OutputTsOffsetOption(ptsOffset, videoTrackTimescale)`. ErsatzTV does the same via `GetLastPtsTime` → `FFmpegState.PtsOffset` → `-output_ts_offset` (emitted only when not copying video). This yields monotonically increasing PTS across program boundaries in a raw TS.
3. **HLS discontinuity tags** (ErsatzTV segmenter, Tunarr HLS modes): the playlist carries `#EXT-X-DISCONTINUITY` (ErsatzTV `TrimPlaylistWithDiscontinuity`), letting compliant players reset their clocks; the TS wrapper (`-c copy -f mpegts`) then re-reads that playlist as a single stream.

Client tolerance: Tunarr's FAQ is blunt — concatenating files with their native near-zero timestamps "produces non-monotonous timestamp sequences that violate MPEG-TS", freezing players. Plex's tuner path is more tolerant of an occasional discontinuity flag than of backwards PTS, but all three projects converged on *never shipping raw per-file timestamps*.

---

## 4. When remux suffices vs when transcode is forced

- **dizqueTV** (`ffmpeg.js`): per-stream copy is the default; transcode is switched on by:
  - `transcodeVideo = opts.normalizeVideoCodec && isDifferentVideoCodec(streamStats.videoCodec, opts.videoEncoder)`
  - `transcodeAudio = opts.normalizeAudioCodec && isDifferentAudioCodec(streamStats.audioCodec, opts.audioEncoder)`
  - forced video transcode whenever any video filter is required: resolution ≠ channel target (scale+pad), watermark overlay, error screens; forced audio transcode when channels/sample-rate normalization (`audioChannelsSampleRate`) or volume/apad filters apply. Settings: `normalizeVideoCodec`, `normalizeAudioCodec`, `normalizeResolution`, `normalizeAudio`.
- **Tunarr** docs/FAQ: heterogeneous libraries mean codec, resolution, fps, audio codec/sample-rate all drift between files; "most hardware decoders cannot switch codecs mid-stream or handle resolution changes"; copy-streaming even matching adjacent files is called impractical because of timestamp non-monotonicity and subtle mismatches (H.264 profile/level, fps variants). Note issue [#1311](https://github.com/chrisbenincasa/tunarr/issues/1311): audio `copy` crashing ffmpeg — copy paths are the fragile ones in practice.
- **ErsatzTV**: makes it a mode choice, not per-file detection — segmenter modes always normalize through a channel-wide FFmpeg Profile ("collections of transcoding settings … applied to all content on a channel"); **HLS Direct** is pure remux and documented as cheaper but boundary-fragile and watermark-less.
- **Mismatch matrix that forces normalization** (union of the three): video codec family, H.264 profile/level, resolution, frame rate, interlacing, pixel format/HDR, audio codec, channel count, sample rate; plus anything requiring a filter (scale, pad, watermark, loudness). If *all* match the channel's declared parameters, `-c copy` remux into fresh TS with rewritten timestamps is sufficient — that is exactly what dizqueTV does file-by-file and what ErsatzTV HLS Direct does channel-wide.
- **Keyframe caveat for remux**: segmented/spliced output wants IDR at boundaries; copied streams have arbitrary GOPs, so joins and program transitions are only clean at keyframes.

---

## 5. Per-tuner-request stream lifecycle

- **dizqueTV** (`video.js`): spawn on HTTP request, `ff.pipe(res, {end: false})`; `res.on('close', …)` → `stop()` → `ffmpeg.kill()`. One concat ffmpeg *per client*; per-program children die as the concat demuxer moves on or on kill.
- **Tunarr**: `SessionManager` keys sessions per channel; `ConnectionTracker` counts clients and tears the session down after the last disconnect (grace period); `ProgramStream.shutdown()` kills the current transcode session. Error items are replaced in-band with generated error streams (`FfmpegStreamFactory.createErrorSession` — testsrc/still + silent audio) so the TS never stops mid-channel.
- **ErsatzTV** (`HlsSessionWorker`): session starts on first tune; runs until cancellation or **idle timeout** (`DateTimeOffset.Now - _lastAccess > timeout`); transcodes item-by-item, switching between **work-ahead** (faster-than-realtime, bounded by a global work-ahead limit semaphore) and **realtime** (`bool realtime = transcodedBuffer >= TimeSpan.FromSeconds(30)`); playlist trimmed to ~last minute with discontinuity preserved; multiple clients share one session. The TS wrapper process is per tuner request, but it is only a `-c copy` remux of the shared session.
- Common pattern relevant to chanarr: **channel session (expensive) is shared and outlives clients briefly; per-client delivery (cheap copy/pipe) is spawned per tuner request and killed on disconnect.**

---

## 6. CPU cost profile

- Remux (`-c copy` demux→mux) is I/O-bound: no decode/encode, typically low single-digit % of one core per channel regardless of resolution. Evidence in-project: ErsatzTV runs its concat/wrapper processes with `-threads 1`, dizqueTV forces `threads=1` for the concat tier, and ErsatzTV docs pitch HLS Direct for "low power systems".
- Software transcode is the dominant cost: roughly one modern CPU core (x264 veryfast) per concurrent 1080p realtime channel, several cores for x265/4K (estimate; scales with resolution/preset). Tunarr's docs flag that HLS-alt's per-program leg "*requires* software encoding" and can stress systems; both Tunarr and ErsatzTV lean on NVENC/QSV/VAAPI to make multi-channel transcode viable (one consumer GPU handles many simultaneous 1080p encodes).
- ErsatzTV's work-ahead scheme shows the real-world shape: transcode bursts faster than realtime to build a ~1-minute buffer, then throttles (`-readrate`/realtime) — so worst-case CPU is at tune-in and program boundaries, not steady-state.
- Implication for chanarr's remux-by-default: a channel whose library is uniform (e.g., all H.264/AAC at one resolution) costs almost nothing per tuner; the moment normalization triggers, budget ~1 core (or a hw-encode slot) per concurrent transcoding channel.

---

## 7. Design implications for chanarr (summary)

1. Use ffmpeg-per-program, never one ffmpeg across heterogeneous inputs; provide continuity either via ffconcat-over-HTTP + `-c copy -f mpegts` (dizqueTV) or via per-item `-output_ts_offset` chained from ffprobe of the previous item's last PTS (Tunarr/ErsatzTV — cleaner, no second ffmpeg tier required if chanarr muxes/splices TS itself).
2. Tune-in seek: input-side `-ss`; frame-accurate when transcoding, keyframe-granular when remuxing — acceptable for TV semantics, but document it.
3. Remux-by-default needs a per-file compatibility gate against the channel's declared stream parameters (codec+profile/level, resolution, fps, audio codec/channels/rate); any mismatch or filter need flips that item (or the channel) to transcode. dizqueTV's `isDifferentVideoCodec`/filter-forcing logic is the proven minimal rule set.
4. Lifecycle: shared per-channel pipeline with idle timeout; per-tuner-request copy fanout; kill on `res close`; keep an error/filler stream generator so the TS never dies mid-session.
