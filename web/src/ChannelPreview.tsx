import { useEffect, useRef, useState } from "react";
import mpegts from "mpegts.js";
import type { Channel } from "./types";
import { api } from "./api";

// Live preview of a channel, played in-browser. chanarr serves MPEG-TS
// (what Plex/HDHomeRun consume), which a <video> element can't decode
// natively — mpegts.js demuxes it via Media Source Extensions and feeds
// <video>. It plays the exact /stream/{number} endpoint the tuner uses, so
// this previews the real thing, not a separate render path.
export function ChannelPreview({
  channel,
  onClose,
}: {
  channel: Channel;
  onClose: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    if (!mpegts.isSupported()) {
      setError("This browser can't play the live stream (no Media Source Extensions).");
      return;
    }

    const player = mpegts.createPlayer(
      { type: "mpegts", url: api.streamUrl(channel.number), isLive: true },
      // chanarr paces the stream at exactly 1x (ffmpeg -re), so the buffer
      // never grows past what has aired. Deliberately do NOT chase the
      // live edge: hugging the leading edge of a 1x stream stalls on the
      // slightest jitter. A preview can sit a few seconds back and play
      // smoothly — latency doesn't matter here. lazyLoad off keeps data
      // flowing so it never pauses the underlying fetch.
      { isLive: true, liveBufferLatencyChasing: false, lazyLoad: false },
    );
    player.on(mpegts.Events.ERROR, (type: string, detail: string) => {
      setError(`Playback error: ${type} — ${detail}`);
    });
    player.attachMediaElement(video);
    player.load();
    const playback = player.play();
    if (playback && typeof playback.catch === "function") {
      // Autoplay can be refused until the user interacts; the controls let
      // them start it manually, so this isn't fatal.
      playback.catch(() => {});
    }

    return () => {
      player.destroy();
    };
  }, [channel.number]);

  // Close on Escape, matching the backdrop click.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="preview-backdrop" onClick={onClose}>
      <div className="preview-modal" onClick={(e) => e.stopPropagation()}>
        <div className="preview-head">
          <div>
            <span className="preview-num">{channel.number}</span>
            <span className="preview-name">{channel.name}</span>
          </div>
          <button className="preview-close" onClick={onClose} aria-label="Close preview">
            &times;
          </button>
        </div>
        {/* eslint-disable-next-line jsx-a11y/media-has-caption -- looping media, no captions */}
        <video ref={videoRef} className="preview-video" controls autoPlay muted playsInline />
        {channel.nowPlaying && <div className="preview-now">Now playing: {channel.nowPlaying}</div>}
        {error && <div className="error-text">{error}</div>}
      </div>
    </div>
  );
}
