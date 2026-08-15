import type {
  Channel,
  PlexConnection,
  PollLinkResult,
  RescanResult,
  ScanResult,
  StartLinkResult,
} from "./types";

class ApiError extends Error {}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const isFormData = init?.body instanceof FormData;
  const res = await fetch(`/api${path}`, {
    ...init,
    headers: isFormData
      ? init?.headers
      : { "Content-Type": "application/json", ...init?.headers },
  });
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // response wasn't JSON; fall back to statusText
    }
    throw new ApiError(message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  scan: (folder: string, credentials?: { username: string; password: string }) =>
    request<ScanResult>("/scan", {
      method: "POST",
      body: JSON.stringify({ folder, ...credentials }),
    }),

  listChannels: () => request<Channel[]>("/channels"),

  createChannel: (input: {
    number: string;
    name: string;
    folder: string;
    shuffle: boolean;
  }) =>
    request<Channel>("/channels", {
      method: "POST",
      body: JSON.stringify({ ...input, shuffleSeed: "0" }),
    }),

  updateChannel: (
    id: number,
    input: { number: string; name: string; shuffle: boolean; shuffleSeed: string },
  ) =>
    request<Channel>(`/channels/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),

  deleteChannel: (id: number) =>
    request<void>(`/channels/${id}`, { method: "DELETE" }),

  rescanChannel: (id: number) =>
    request<RescanResult>(`/channels/${id}/rescan`, { method: "POST" }),

  uploadLogo: (id: number, file: File) => {
    const form = new FormData();
    form.append("logo", file);
    return request<Channel>(`/channels/${id}/logo`, {
      method: "POST",
      body: form,
    });
  },

  logoUrl: (id: number) => `/api/channels/${id}/logo`,

  // The live MPEG-TS stream a tuner client (or the in-app preview) plays.
  // Not under /api — it's the same endpoint Plex's HDHomeRun lineup points
  // at. Keyed by channel number, matching the tuner lineup.
  streamUrl: (number: string) => `/stream/${number}`,

  startPlexLink: () =>
    request<StartLinkResult>("/plex/link/start", { method: "POST" }),

  pollPlexLink: (pinId: number) =>
    request<PollLinkResult>(`/plex/link/poll?pinId=${pinId}`),

  saveConnection: (pinId: number, serverUrl: string) =>
    request<{ connected: boolean }>("/plex/connection", {
      method: "POST",
      body: JSON.stringify({ pinId, serverUrl }),
    }),

  getConnection: () => request<PlexConnection>("/plex/connection"),
};

export { ApiError };
