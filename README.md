# chanarr

Turns folders of local media into looping virtual-timeline TV channels that Plex detects and plays via HDHomeRun tuner emulation.

Full spec: [.scratch/chanarr-spec/spec.md](.scratch/chanarr-spec/spec.md). Domain vocabulary: [CONTEXT.md](CONTEXT.md). Architecture decisions: [docs/adr/](docs/adr/).

## Status

v1 complete: backend and frontend both implemented and wired together, verified end to end (create a channel through the UI, it streams real video to a tuner client).

## Requirements

- Go 1.26+
- Node 24+ (frontend)
- `ffmpeg` / `ffprobe` on PATH (checked at startup)

## Layout

```
cmd/chanarr/       entrypoint
internal/schedule/ Channel/Epoch/Airing domain model, ProgramAt (docs/adr/0001)
internal/tuner/    HDHomeRun emulation (discover/lineup/lineup_status/device.xml)
internal/guide/    XMLTV guide generation + Plex reloadGuide push
internal/stream/   per-channel MPEG-TS streaming (ffmpeg)
internal/library/  folder scanning, SxxExx metadata parsing
internal/store/    SQLite persistence
internal/plexlink/ optional PIN-link Plex connection (docs/adr/0002)
internal/httpapi/  REST API for the web UI
internal/webui/    embeds the built frontend into the Go binary
internal/config/   startup config, ffmpeg presence check
web/               React + TypeScript UI (Vite)
```

## Develop

Two dev servers, run separately — Vite proxies `/api` to the Go server (`web/vite.config.ts`):

```bash
go run ./cmd/chanarr
```

```bash
cd web && npm install && npm run dev
```

## Build a single binary

The frontend build output is embedded into the Go binary (`internal/webui`), so build the frontend first:

```bash
cd web && npm install && npm run build
cd .. && go build -o chanarr ./cmd/chanarr
./chanarr
```
