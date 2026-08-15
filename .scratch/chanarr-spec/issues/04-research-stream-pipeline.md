# Research: continuous MPEG-TS stream pipeline

Type: research
Status: resolved

## Question

How do dizqueTV/Tunarr and ErsatzTV turn a playlist of heterogeneous files into one continuous MPEG-TS stream a tuner client will accept? ffmpeg invocation patterns (concat vs per-program restart), joining mid-file for virtual-timeline tune-in (`-ss` accuracy), keeping PTS/DTS continuous across file boundaries, when remux suffices vs when transcode/normalization is forced (codec, resolution, audio mismatches), stream lifecycle per tuner request, and CPU cost profile.

## Answer

Full findings with source links and example command lines: [assets/research-stream-pipeline.md](../assets/research-stream-pipeline.md)

- **No project uses a single long-lived ffmpeg across files.** All three run one ffmpeg per program item plus a continuity layer: dizqueTV pipes an ffconcat playlist of localhost per-program streams through a second `-f concat … -c copy -f mpegts pipe:1` process per client; ErsatzTV (and Tunarr's default mode) run a per-channel HLS session (one ffmpeg per item writing segments) and serve tuners by remuxing their own HLS playlist with `-c copy -f mpegts`.
- **Mid-file tune-in** is an input-side `-ss <wall-clock offset>` on the first item only: frame-accurate when transcoding (decode-and-discard), keyframe-granular and boundary-glitchy when stream-copying — the documented reason copy modes (ErsatzTV HLS Direct, ts-legacy) "have issues at program boundaries".
- **PTS/DTS continuity**: either the concat demuxer's automatic timestamp rewriting (+ `-fflags +genpts+igndts`), or explicitly chaining `-output_ts_offset` per item from an ffprobe of the previous output's last packet PTS (Tunarr `GetLastPtsDuration`, ErsatzTV `GetLastPtsTime`), or HLS `#EXT-X-DISCONTINUITY` tags. Raw per-file timestamps are never shipped — non-monotonic PTS freezes players.
- **Remux vs transcode**: copy suffices only when the file matches the channel's declared parameters (codec + H.264 profile/level, resolution, fps, audio codec/channels/rate) and no filter (scale/pad/watermark) is needed; dizqueTV's `isDifferentVideoCodec`/`isDifferentAudioCodec` + filter-forcing rules are the proven minimal gate. Tunarr defaults to always-normalize; ErsatzTV makes it a per-channel mode (HLS Direct = remux).
- **Lifecycle**: shared per-channel session with idle timeout (ErsatzTV/Tunarr) or per-client spawn (dizqueTV); kill on HTTP disconnect; error/filler generator streams keep the TS alive on item failure. ErsatzTV throttles with work-ahead-then-realtime (`-readrate`) buffering ~30–60 s.
- **CPU**: remux is I/O-bound (low single-digit % of a core per channel, `-threads 1` everywhere in copy paths); software transcode costs roughly a core per concurrent 1080p channel (x264 veryfast, estimate), hence both projects' hardware-encode support. Cost concentrates at tune-in/boundaries under work-ahead scheduling.
