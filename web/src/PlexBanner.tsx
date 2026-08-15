import { useEffect, useRef, useState } from "react";
import { api } from "./api";

const DISMISS_KEY = "chanarr-plex-banner-dismissed";

type Stage = "idle" | "starting" | "pending" | "linked" | "saving";

// Optional, prompted connection to Plex for instant guide-reload pushes —
// never required at onboarding, a channel plays fine without it
// (docs/adr/0002). Dismissible; once dismissed it stays dismissed rather
// than nagging on every reload.
export function PlexBanner() {
  const [connected, setConnected] = useState<boolean | null>(null);
  const [dismissed, setDismissed] = useState(
    () => localStorage.getItem(DISMISS_KEY) === "1",
  );
  const [stage, setStage] = useState<Stage>("idle");
  const [code, setCode] = useState("");
  const [pinId, setPinId] = useState<number | null>(null);
  const [serverUrl, setServerUrl] = useState("");
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    api
      .getConnection()
      .then((c) => setConnected(c.connected))
      .catch(() => setConnected(false));
  }, []);

  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  function dismiss() {
    localStorage.setItem(DISMISS_KEY, "1");
    setDismissed(true);
  }

  async function startLink() {
    setError("");
    setStage("starting");
    try {
      const result = await api.startPlexLink();
      setPinId(result.pinId);
      setCode(result.code);
      setStage("pending");
      pollRef.current = setInterval(async () => {
        try {
          const poll = await api.pollPlexLink(result.pinId);
          if (poll.linked) {
            if (pollRef.current) clearInterval(pollRef.current);
            setStage("linked");
          }
        } catch {
          // transient poll failure — keep trying until the interval is cleared
        }
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't start the Plex link");
      setStage("idle");
    }
  }

  async function finishConnecting() {
    if (!pinId || !serverUrl.trim()) return;
    setError("");
    setStage("saving");
    try {
      await api.saveConnection(pinId, serverUrl.trim());
      setConnected(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't save the connection");
      setStage("linked");
    }
  }

  if (connected === null || connected === true || dismissed) return null;

  if (stage === "idle" || stage === "starting") {
    return (
      <div className="plex-banner">
        <span>Guide updates rely on Plex&rsquo;s slow daily refresh.</span>
        <div className="actions">
          <button className="link" onClick={startLink} disabled={stage === "starting"}>
            {stage === "starting" ? "Starting…" : "Connect Plex"}
          </button>
          <button className="dismiss" onClick={dismiss} aria-label="Dismiss">
            &times;
          </button>
        </div>
        {error && <div className="error-text">{error}</div>}
      </div>
    );
  }

  if (stage === "pending") {
    return (
      <div className="plex-banner">
        <span>
          Enter <span className="code">{code}</span> at{" "}
          <a href="https://plex.tv/link" target="_blank" rel="noopener noreferrer">
            plex.tv/link
          </a>
        </span>
        <button className="dismiss" onClick={dismiss} aria-label="Dismiss">
          &times;
        </button>
      </div>
    );
  }

  // linked or saving: plex.tv only authenticates the account — it doesn't
  // tell chanarr which local server to push guide reloads to.
  return (
    <div className="plex-banner">
      <form
        className="plex-connect-form"
        onSubmit={(e) => {
          e.preventDefault();
          finishConnecting();
        }}
      >
        <input
          type="text"
          placeholder="http://192.168.1.50:32400"
          value={serverUrl}
          onChange={(e) => setServerUrl(e.target.value)}
          disabled={stage === "saving"}
        />
        <button className="link" type="submit" disabled={stage === "saving" || !serverUrl.trim()}>
          {stage === "saving" ? "Saving…" : "Connect"}
        </button>
      </form>
      {error && <div className="error-text">{error}</div>}
    </div>
  );
}
