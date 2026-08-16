# chanarr

Turns folders of local media into looping virtual-timeline TV channels that Plex detects and plays via HDHomeRun tuner emulation.

Full spec: [.scratch/chanarr-spec/spec.md](.scratch/chanarr-spec/spec.md). Domain vocabulary: [CONTEXT.md](CONTEXT.md). Architecture decisions: [docs/adr/](docs/adr/).

## Status

v1 complete: backend and frontend both implemented and wired together, verified end to end (create a channel through the UI, it streams real video to a tuner client).

## Media locations

A channel's folder can be a local path or a network share — no OS-level mount required:

- Local: `/media/tv/My Show`
- SMB/CIFS: `smb://nas/media/My Show` (optional username/password in the wizard; blank = guest)
- NFS v3: `nfs://nas/volume1/media/My Show` (access is governed by the server's export rules — no password)

ffmpeg/ffprobe can't read SMB/NFS directly, so chanarr streams remote files to them through a loopback HTTP bridge with byte-range support (`internal/netfs`). SMB logins are stored keyed by host+share and reused for rescans and streaming.

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
internal/netfs/    local/SMB/NFS folder access + loopback HTTP bridge for ffmpeg
internal/store/    SQLite persistence
internal/plexlink/ optional PIN-link Plex connection (docs/adr/0002)
internal/httpapi/  REST API for the web UI
internal/webui/    embeds the built frontend into the Go binary
internal/config/   startup config, ffmpeg presence check
web/               React + TypeScript UI (Vite)
```

## Data location

Everything persistent (channels, Plex connection, share logins, uploaded logos) lives in one per-user data directory, regardless of where the server is launched from:

- macOS: `~/Library/Application Support/chanarr`
- Linux: `~/.config/chanarr`

Override with `CHANARR_DATA_DIR` (throwaway dev runs, Docker volumes, multiple instances); `CHANARR_ADDR` overrides the listen address (default `:5004`). A `chanarr.db` left in the launch directory by older builds is copied into the data dir on first run.

## Deploy to Proxmox

One command on the Proxmox host, [community-scripts](https://community-scripts.org/) style:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/brian-denton/chanarr/main/ct/chanarr.sh)"
```

A dialog offers default settings (unprivileged Debian 12 LXC, 2 cores / 1GB / 4GB, DHCP on vmbr0) or advanced ones (CTID, hostname, sizing, bridge, static IP, storage). It downloads the Debian template if needed, installs ffmpeg, fetches the latest chanarr release binary, and runs it as a systemd service (`chanarr.service`, data in `/var/lib/chanarr`, starts on boot), then prints the URL.

**Updating:** run the same one-liner *inside* the container — it detects an existing install and swaps in the latest release.

NAS media needs no mounts — point channels at `smb://`/`nfs://` folders; chanarr's share clients are pure userspace, which is why an unprivileged container is enough. Media on the Proxmox host itself can be bind-mounted: `pct set <ctid> -mp0 /path/on/host,mp=/media,ro=1`.

**Publishing a release** (what the installer downloads):

```bash
./deploy/release.sh v0.1.0
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
