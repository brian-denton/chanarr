import { useRef, useState } from "react";
import { api } from "./api";
import type { Channel } from "./types";

export function EditChannelPanel({
  channel,
  onUpdated,
  onDeleted,
  onBack,
}: {
  channel: Channel;
  onUpdated: (channel: Channel) => void;
  onDeleted: (id: number) => void;
  onBack: () => void;
}) {
  const [name, setName] = useState(channel.name);
  const [number, setNumber] = useState(channel.number);
  const [shuffle, setShuffle] = useState(channel.shuffle);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  const [logoBust, setLogoBust] = useState(0);
  const [rescanStatus, setRescanStatus] = useState("");
  const [rescanning, setRescanning] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  async function handleSave() {
    if (!name.trim() || !number.trim()) return;
    setError("");
    setSaving(true);
    try {
      const updated = await api.updateChannel(channel.id, {
        number: number.trim(),
        name: name.trim(),
        shuffle,
        shuffleSeed: channel.shuffleSeed,
      });
      onUpdated(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't save changes");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    setError("");
    setDeleting(true);
    try {
      await api.deleteChannel(channel.id);
      onDeleted(channel.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't delete this channel");
      setDeleting(false);
    }
  }

  async function handleLogoChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError("");
    try {
      const updated = await api.uploadLogo(channel.id, file);
      setLogoBust((n) => n + 1);
      onUpdated(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't upload that image");
    } finally {
      if (fileInput.current) fileInput.current.value = "";
    }
  }

  async function handleRescan() {
    setError("");
    setRescanning(true);
    setRescanStatus("");
    try {
      const result = await api.rescanChannel(channel.id);
      setRescanStatus(
        result.changed
          ? `Updated — ${result.episodeCount} episodes now.`
          : "No changes found.",
      );
    } catch (err) {
      setError(err instanceof Error ? err.message : "Rescan failed");
    } finally {
      setRescanning(false);
    }
  }

  return (
    <div className="edit-panel">
      <h2>Edit channel</h2>
      {error && <div className="error-text">{error}</div>}

      <div className="field">
        <label htmlFor="edit-name">Name</label>
        <input id="edit-name" type="text" value={name} onChange={(e) => setName(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor="edit-number">Channel number</label>
        <input id="edit-number" type="text" value={number} onChange={(e) => setNumber(e.target.value)} />
      </div>

      <div className="field">
        <label>Logo</label>
        <div className="logo-row">
          {channel.hasLogo ? (
            <img
              className="preview"
              src={`${api.logoUrl(channel.id)}?v=${logoBust}`}
              alt=""
            />
          ) : (
            <span className="preview-fallback" aria-hidden="true">
              📺
            </span>
          )}
          <span className="status">
            {channel.hasLogo ? "Auto-detected or uploaded" : "No logo set"}
          </span>
          <button type="button" onClick={() => fileInput.current?.click()}>
            Upload…
          </button>
          <input
            ref={fileInput}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            style={{ display: "none" }}
            onChange={handleLogoChange}
          />
        </div>
      </div>

      <div className="toggle-row">
        <span>Shuffle order</span>
        <button
          type="button"
          className={`switch ${shuffle ? "on" : ""}`}
          role="switch"
          aria-checked={shuffle}
          onClick={() => setShuffle((v) => !v)}
        />
      </div>

      <div className="row-actions">
        <button className="cta" onClick={handleSave} disabled={saving || !name.trim() || !number.trim()}>
          {saving ? "Saving…" : "Save"}
        </button>
        <button className="danger" onClick={handleDelete} disabled={deleting}>
          {deleting ? "Deleting…" : "Delete"}
        </button>
      </div>

      <div className="rescan-status">
        <button className="ghost" onClick={handleRescan} disabled={rescanning}>
          {rescanning ? "Rescanning…" : "Rescan folder now"}
        </button>
        {rescanStatus && <div>{rescanStatus}</div>}
      </div>

      <button className="ghost" onClick={onBack}>
        Back
      </button>
    </div>
  );
}
