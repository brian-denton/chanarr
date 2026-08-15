import type { Channel } from "./types";
import { api } from "./api";

export function Home({
  channels,
  onSelectChannel,
  onAddChannel,
}: {
  channels: Channel[];
  onSelectChannel: (id: number) => void;
  onAddChannel: () => void;
}) {
  return (
    <div className="channel-list">
      {channels.length === 0 ? (
        <div className="empty-state">No channels yet — point at a folder to create your first one.</div>
      ) : (
        channels.map((ch) => (
          <button
            key={ch.id}
            className="channel-row"
            onClick={() => onSelectChannel(ch.id)}
          >
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
        ))
      )}
      <div className="add-row">
        <button onClick={onAddChannel}>+ Point at a new folder</button>
      </div>
    </div>
  );
}
