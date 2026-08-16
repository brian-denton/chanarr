#!/usr/bin/env bash
# chanarr Proxmox installer — run this ON THE PROXMOX HOST as root.
#
# Creates an unprivileged Debian 12 LXC container, installs ffmpeg, pushes
# the chanarr binary in, and runs it as a systemd service that starts with
# the container (and the container starts with the host).
#
# Usage:
#   1. On your dev machine:  ./deploy/build-release.sh
#   2. Copy to the host:     scp dist/chanarr-linux-amd64 deploy/proxmox-install.sh root@proxmox:
#   3. On the host:          ./proxmox-install.sh                  # fresh install
#                            ./proxmox-install.sh update <ctid>    # push a new binary into an existing container
#
# Tunables (env vars, all optional):
#   CTID        container id           (default: next free id)
#   HOSTNAME    container hostname     (default: chanarr)
#   STORAGE     rootfs storage         (default: local-lvm)
#   TEMPLATE_STORAGE  template storage (default: local)
#   BRIDGE      network bridge         (default: vmbr0)
#   IP          container ip           (default: dhcp; or e.g. 192.168.1.50/24,gw=192.168.1.1)
#   DISK_GB     rootfs size in GB      (default: 4)
#   MEMORY_MB   memory in MB           (default: 1024)
#   CORES       cpu cores              (default: 2)
#   BINARY      path to chanarr binary (default: ./chanarr-linux-amd64)
#
# Media on a NAS needs no mounts at all — point channels at smb:// or
# nfs:// folders in the UI; chanarr's clients are pure userspace, which is
# exactly why an unprivileged container suffices. To use media that lives
# on the Proxmox host itself, bind-mount it after install:
#   pct set <ctid> -mp0 /path/on/host,mp=/media,ro=1
set -euo pipefail

BINARY="${BINARY:-./chanarr-linux-amd64}"
HOSTNAME="${HOSTNAME:-chanarr}"
STORAGE="${STORAGE:-local-lvm}"
TEMPLATE_STORAGE="${TEMPLATE_STORAGE:-local}"
BRIDGE="${BRIDGE:-vmbr0}"
IP="${IP:-dhcp}"
DISK_GB="${DISK_GB:-4}"
MEMORY_MB="${MEMORY_MB:-1024}"
CORES="${CORES:-2}"

die() { echo "error: $*" >&2; exit 1; }

command -v pct >/dev/null || die "pct not found — run this on the Proxmox host"
[ "$(id -u)" = 0 ] || die "run as root"
[ -f "$BINARY" ] || die "chanarr binary not found at $BINARY (build with deploy/build-release.sh, or set BINARY=)"

# install_into pushes the binary and service unit into a running container
# and (re)starts the service — shared by fresh installs and updates.
install_into() {
	local ctid="$1"

	pct exec "$ctid" -- bash -c '
		set -e
		export DEBIAN_FRONTEND=noninteractive
		if ! command -v ffmpeg >/dev/null; then
			apt-get update -qq
			apt-get install -y -qq --no-install-recommends ffmpeg ca-certificates >/dev/null
		fi
		id chanarr >/dev/null 2>&1 || useradd --system --home-dir /var/lib/chanarr --shell /usr/sbin/nologin chanarr
		mkdir -p /var/lib/chanarr
		chown chanarr:chanarr /var/lib/chanarr
	'

	pct exec "$ctid" -- systemctl stop chanarr 2>/dev/null || true
	pct push "$ctid" "$BINARY" /usr/local/bin/chanarr --perms 0755

	local unit
	unit="$(mktemp)"
	cat > "$unit" <<'EOF'
[Unit]
Description=chanarr - virtual TV channels for Plex
After=network-online.target
Wants=network-online.target

[Service]
User=chanarr
Group=chanarr
Environment=CHANARR_DATA_DIR=/var/lib/chanarr
ExecStart=/usr/local/bin/chanarr
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
	pct push "$ctid" "$unit" /etc/systemd/system/chanarr.service --perms 0644
	rm -f "$unit"

	pct exec "$ctid" -- bash -c 'systemctl daemon-reload && systemctl enable --now chanarr'
}

report() {
	local ctid="$1"
	local ip
	ip="$(pct exec "$ctid" -- hostname -I | awk "{print \$1}")"
	echo
	echo "chanarr is running in container $ctid"
	echo "  Web UI:      http://$ip:5004"
	echo "  Plex tuner:  http://$ip:5004  (HDHomeRun autodiscovery on the same address)"
	echo "  XMLTV guide: http://$ip:5004/epg.xml"
	echo "  Data dir:    /var/lib/chanarr (inside the container)"
	echo "  Logs:        pct exec $ctid -- journalctl -u chanarr -f"
}

# ---- update mode -----------------------------------------------------------
if [ "${1:-}" = "update" ]; then
	CTID="${2:-}"
	[ -n "$CTID" ] || die "usage: $0 update <ctid>"
	pct status "$CTID" >/dev/null 2>&1 || die "container $CTID not found"
	[ "$(pct status "$CTID")" = "status: running" ] || pct start "$CTID"
	install_into "$CTID"
	report "$CTID"
	exit 0
fi

# ---- fresh install ---------------------------------------------------------
CTID="${CTID:-$(pvesh get /cluster/nextid)}"
pct status "$CTID" >/dev/null 2>&1 && die "container $CTID already exists (use: $0 update $CTID)"

# Find a Debian 12 template, downloading the newest one if none is local.
TEMPLATE="$(pveam list "$TEMPLATE_STORAGE" 2>/dev/null | awk '/debian-12-standard/{print $1}' | sort -V | tail -1)"
if [ -z "$TEMPLATE" ]; then
	echo "downloading Debian 12 template..."
	pveam update >/dev/null
	LATEST="$(pveam available --section system | awk '/debian-12-standard/{print $2}' | sort -V | tail -1)"
	[ -n "$LATEST" ] || die "no debian-12-standard template available via pveam"
	pveam download "$TEMPLATE_STORAGE" "$LATEST"
	TEMPLATE="$TEMPLATE_STORAGE:vztmpl/$LATEST"
fi

NET0="name=eth0,bridge=$BRIDGE,ip=$IP"
if [ "$IP" = "dhcp" ]; then
	NET0="name=eth0,bridge=$BRIDGE,ip=dhcp"
fi

echo "creating container $CTID ($HOSTNAME) from $TEMPLATE..."
pct create "$CTID" "$TEMPLATE" \
	--hostname "$HOSTNAME" \
	--memory "$MEMORY_MB" \
	--cores "$CORES" \
	--rootfs "$STORAGE:$DISK_GB" \
	--net0 "$NET0" \
	--unprivileged 1 \
	--features nesting=1 \
	--onboot 1 \
	--start 1

# Wait for the network before apt needs it.
echo "waiting for container network..."
for _ in $(seq 1 30); do
	if pct exec "$CTID" -- ping -c1 -W1 deb.debian.org >/dev/null 2>&1; then
		break
	fi
	sleep 2
done

install_into "$CTID"
report "$CTID"
