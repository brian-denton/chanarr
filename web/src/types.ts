// Mirrors internal/httpapi's JSON shapes exactly.

export interface Channel {
  id: number;
  number: string;
  name: string;
  folder: string;
  shuffle: boolean;
  // int64 on the wire as a JSON string (see internal/httpapi/helpers.go) —
  // shuffle seeds are drawn from the full int64 range and routinely exceed
  // 2^53, the largest integer a JS number can hold exactly.
  shuffleSeed: string;
  hasLogo: boolean;
  episodeCount: number;
  nowPlaying?: string;
}

export interface ScanResult {
  name: string;
  number: string;
  episodeCount: number;
  hasLogo: boolean;
}

export interface RescanResult {
  changed: boolean;
  episodeCount: number;
}

export interface PlexConnection {
  serverUrl: string;
  connected: boolean;
}

export interface StartLinkResult {
  pinId: number;
  code: string;
}

export interface PollLinkResult {
  linked: boolean;
}

export type Theme = "system" | "light" | "dark";
