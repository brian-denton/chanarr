import { useEffect, useState } from "react";
import { api } from "./api";
import type { Channel } from "./types";
import { useTheme } from "./useTheme";
import { ThemeToggle } from "./ThemeToggle";
import { PlexBanner } from "./PlexBanner";
import { Home } from "./Home";
import { AddChannelWizard } from "./AddChannelWizard";
import { EditChannelPanel } from "./EditChannelPanel";

type Screen = "list" | "add" | "edit";

function App() {
  const [theme, setTheme] = useTheme();
  const [screen, setScreen] = useState<Screen>("list");
  const [channels, setChannels] = useState<Channel[]>([]);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    refreshChannels();
  }, []);

  async function refreshChannels() {
    setLoading(true);
    setError("");
    try {
      setChannels(await api.listChannels());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't load channels");
    } finally {
      setLoading(false);
    }
  }

  function handleCreated(channel: Channel) {
    setChannels((prev) => [...prev, channel]);
    setScreen("list");
  }

  function handleUpdated(channel: Channel) {
    setChannels((prev) => prev.map((c) => (c.id === channel.id ? channel : c)));
    setScreen("list");
  }

  function handleDeleted(id: number) {
    setChannels((prev) => prev.filter((c) => c.id !== id));
    setScreen("list");
  }

  const editingChannel = channels.find((c) => c.id === editingId) ?? null;

  return (
    <div className="app">
      <div className="app-wrap">
        <div className="topbar">
          <div className="brand">
            chan<span>arr</span>
          </div>
          <ThemeToggle theme={theme} onChange={setTheme} />
        </div>

        <PlexBanner />

        {loading && <div className="empty-state">Loading&hellip;</div>}
        {!loading && error && <div className="error-text">{error}</div>}

        {!loading && !error && screen === "list" && (
          <Home
            channels={channels}
            onSelectChannel={(id) => {
              setEditingId(id);
              setScreen("edit");
            }}
            onAddChannel={() => setScreen("add")}
          />
        )}

        {screen === "add" && (
          <AddChannelWizard onCreated={handleCreated} onCancel={() => setScreen("list")} />
        )}

        {screen === "edit" && editingChannel && (
          <EditChannelPanel
            channel={editingChannel}
            onUpdated={handleUpdated}
            onDeleted={handleDeleted}
            onBack={() => setScreen("list")}
          />
        )}
      </div>
    </div>
  );
}

export default App;
