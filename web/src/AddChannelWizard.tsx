import { useState } from "react";
import { api } from "./api";
import type { Channel, ScanResult } from "./types";

export function AddChannelWizard({
  onCreated,
  onCancel,
}: {
  onCreated: (channel: Channel) => void;
  onCancel: () => void;
}) {
  const [folder, setFolder] = useState("");
  const [scanning, setScanning] = useState(false);
  const [scan, setScan] = useState<ScanResult | null>(null);
  const [name, setName] = useState("");
  const [number, setNumber] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  async function handleScan() {
    if (!folder.trim()) return;
    setError("");
    setScanning(true);
    try {
      const result = await api.scan(folder.trim());
      setScan(result);
      setName(result.name);
      setNumber(result.number);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't scan that folder");
    } finally {
      setScanning(false);
    }
  }

  async function handleCreate() {
    if (!name.trim() || !number.trim()) return;
    setError("");
    setCreating(true);
    try {
      const channel = await api.createChannel({
        number: number.trim(),
        name: name.trim(),
        folder: folder.trim(),
        shuffle: false,
      });
      onCreated(channel);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't create the channel");
      setCreating(false);
    }
  }

  if (!scan) {
    return (
      <div className="step">
        <div className="icon" aria-hidden="true">
          📁
        </div>
        <h2>Point at a folder</h2>
        <p>
          Pick the folder with this show&rsquo;s seasons in it. chanarr scans it and
          loops everything on its own channel.
        </p>
        {error && <div className="error-text">{error}</div>}
        <input
          type="text"
          placeholder="/media/tv/My Show"
          value={folder}
          onChange={(e) => setFolder(e.target.value)}
          disabled={scanning}
        />
        <button className="cta" onClick={handleScan} disabled={!folder.trim() || scanning}>
          {scanning ? "Scanning…" : "Scan folder"}
        </button>
        <button className="ghost" onClick={onCancel}>
          Cancel
        </button>
      </div>
    );
  }

  return (
    <div className="step">
      <div className="icon" aria-hidden="true">
        ✅
      </div>
      <h2>Found {scan.episodeCount} episodes</h2>
      <div className="scan-result">
        Channel name: <b>{name}</b>
        <br />
        Channel number: <b>{number}</b>
        <br />
        {scan.hasLogo && (
          <>
            Logo: <b>auto-detected</b>
          </>
        )}
      </div>
      <div className="field">
        <label htmlFor="wizard-name">Name</label>
        <input
          id="wizard-name"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>
      <div className="field">
        <label htmlFor="wizard-number">Channel number</label>
        <input
          id="wizard-number"
          type="text"
          value={number}
          onChange={(e) => setNumber(e.target.value)}
        />
      </div>
      {error && <div className="error-text">{error}</div>}
      <button className="cta" onClick={handleCreate} disabled={creating || !name.trim() || !number.trim()}>
        {creating ? "Creating…" : "Create channel"}
      </button>
      <button className="ghost" onClick={onCancel}>
        Cancel
      </button>
    </div>
  );
}
