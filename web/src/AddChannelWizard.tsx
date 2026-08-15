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
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [scanning, setScanning] = useState(false);
  const [scan, setScan] = useState<ScanResult | null>(null);
  const [name, setName] = useState("");
  const [number, setNumber] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  // Network shares (smb:// or nfs://) can take an optional login. SMB uses
  // both fields; NFS has no password auth (its access comes from the
  // server's export rules) so only the hint copy differs.
  const trimmed = folder.trim();
  const isSMB = trimmed.startsWith("smb://");
  const isNFS = trimmed.startsWith("nfs://");

  async function handleScan() {
    if (!trimmed) return;
    setError("");
    setScanning(true);
    try {
      const credentials =
        isSMB && (username || password) ? { username, password } : undefined;
      const result = await api.scan(trimmed, credentials);
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
        <p className="hint">
          Local path, or a network share:{" "}
          <code>smb://nas/media/My Show</code> or <code>nfs://nas/volume1/My Show</code>.
        </p>
        {isSMB && (
          <div className="share-credentials">
            <input
              type="text"
              placeholder="Username (optional — blank for guest)"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={scanning}
              autoComplete="off"
            />
            <input
              type="password"
              placeholder="Password (optional)"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={scanning}
              autoComplete="off"
            />
          </div>
        )}
        {isNFS && (
          <p className="hint">
            NFS access is granted by the server&rsquo;s export rules — no username or
            password needed here.
          </p>
        )}
        <button className="cta" onClick={handleScan} disabled={!trimmed || scanning}>
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
