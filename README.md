# chanarr

Turns folders of local media into looping virtual-timeline TV channels that Plex detects and plays via HDHomeRun tuner emulation.

Full spec: [.scratch/chanarr-spec/spec.md](.scratch/chanarr-spec/spec.md). Domain vocabulary: [CONTEXT.md](CONTEXT.md). Architecture decisions: [docs/adr/](docs/adr/).

## Status

Scaffolded, not yet implemented. Package layout mirrors the spec's sections (see doc comments in each `internal/*` package for what belongs there and what's still TODO). The HDHomeRun tuner-discovery endpoints (`internal/tuner`) are already wired up and match the behavior proven against a real Plex server in the walking-skeleton prototype.

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
internal/config/   startup config, ffmpeg presence check
web/               React + TypeScript UI (Vite)
```

## Run

```bash
go run ./cmd/chanarr
```

```bash
cd web && npm install && npm run dev
```
