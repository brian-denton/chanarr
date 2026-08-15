import { useState } from "react";
import type { Channel } from "./types";
import { api } from "./api";
import { ChannelPreview } from "./ChannelPreview";

export function Home({
  channels,
  onSelectChannel,
  onAddChannel,
}: {
  channels: Channel[];
  onSelectChannel: (id: number) => void;
  onAddChannel: () => void;
}) {
  const [previewing, setPreviewing] = useState<Channel | null>(null);

  return (
    <div className="channel-list">
      {channels.length === 0 ? (
        <div className="empty-state">No channels yet — point at a folder to create your first one.</div>
      ) : (
        channels.map((ch) => (
          // A row is two separate controls (edit vs. preview) rather than a
          // button-in-button, which isn't valid HTML.
          <div key={ch.id} className="channel-row">
            <button className="channel-main" onClick={() => onSelectChannel(ch.id)}>
              <span className="num">{ch.number}</span>
              {ch.hasLogo ? (
                <img className="logo" src={api.logoUrl(ch.id)} alt="" />
              ) : (
                <span className="logo-fallback" aria-hidden="true">
                  📺
                </span>
              )}
              <div className="meta">
                <div className="name">{ch.name}</div>
                <div className="now">{ch.nowPlaying || `${ch.episodeCount} episodes`}</div>
              </div>
            </button>
            <button
              className="preview-btn"
              onClick={() => setPreviewing(ch)}
              aria-label={`Preview ${ch.name}`}
              title="Live preview"
            >
              ▶
            </button>
          </div>
        ))
      )}
      <div className="add-row">
        <button onClick={onAddChannel}>+ Point at a new folder</button>
      </div>

      {previewing && (
        <ChannelPreview channel={previewing} onClose={() => setPreviewing(null)} />
      )}
    </div>
  );
}
