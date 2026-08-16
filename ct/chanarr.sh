#!/usr/bin/env bash
# chanarr LXC installer — community-scripts.org style.
#
# On the Proxmox host, run:
#   bash -c "$(curl -fsSL https://raw.githubusercontent.com/brian-denton/chanarr/main/ct/chanarr.sh)"
#
# Creates an unprivileged Debian 12 container and installs chanarr as a
# systemd service (binary from this repo's GitHub releases). Run the same
# command again INSIDE the container to update chanarr to the latest
# release.

set -euo pipefail

REPO="brian-denton/chanarr"
RAW="https://raw.githubusercontent.com/${REPO}/main"
APP="chanarr"
LOG="/tmp/chanarr-install.log"

YW=$'\033[33m'
GN=$'\033[1;92m'
RD=$'\033[01;31m'
BL=$'\033[36m'
CL=$'\033[m'

msg_info() { echo -e "${YW}⏳ $1${CL}"; }
msg_ok() { echo -e "${GN}✔️  $1${CL}"; }
msg_error() { echo -e "${RD}✖️  $1${CL}"; }

fail() {
	msg_error "$1"
	[ -s "$LOG" ] && { echo "--- last output ($LOG):"; tail -5 "$LOG"; }
	exit 1
}

header() {
	cat <<'EOF'
        __
  _____/ /_  ____ _____  ____ ___________
 / ___/ __ \/ __ `/ __ \/ __ `/ ___/ ___/
/ /__/ / / / /_/ / / / / /_/ / /  / /
\___/_/ /_/\__,_/_/ /_/\__,_/_/  /_/

  folders of media -> live TV channels in Plex
EOF
	echo
}

# ---- update mode: the same one-liner, run inside the container -------------
if [ ! -e /dev/pve ] && ! command -v pct >/dev/null 2>&1; then
	if [ -f /etc/systemd/system/chanarr.service ]; then
		header
		msg_info "Updating ${APP} to the latest release"
		curl -fsSL "https://github.com/${REPO}/releases/latest/download/chanarr-linux-amd64" \
			-o /usr/local/bin/chanarr.new >"$LOG" 2>&1 || fail "download failed"
		chmod 0755 /usr/local/bin/chanarr.new
		mv /usr/local/bin/chanarr.new /usr/local/bin/chanarr
		systemctl restart chanarr
		msg_ok "Updated ${APP} and restarted the service"
		exit 0
	fi
	msg_error "This doesn't look like a Proxmox host (no pct) or a chanarr container."
	exit 1
fi

# ---- create mode: on the Proxmox host ---------------------------------------
[ "$(id -u)" = 0 ] || { msg_error "Run as root on the Proxmox host."; exit 1; }
command -v pveversion >/dev/null || { msg_error "This must run on a Proxmox VE host."; exit 1; }

header

# Defaults (Advanced Settings can change all of them).
CTID="$(pvesh get /cluster/nextid)"
HOSTNAME="chanarr"
DISK_GB="4"
CORES="2"
MEMORY_MB="1024"
BRIDGE="vmbr0"
NET_IP="dhcp"
GATEWAY=""

ask() { whiptail --backtitle "chanarr LXC" --title "$1" --inputbox "$2" 10 60 "$3" 3>&1 1>&2 2>&3; }

# choose_storage <content-type> <title> — whiptail menu over the storages
# that support that content, auto-picking when there's only one.
choose_storage() {
	local content="$1" title="$2"
	local -a items=()
	local line
	while read -r line; do
		items+=("$line" "")
	done < <(pvesm status -content "$content" | awk 'NR>1 {print $1}')
	[ "${#items[@]}" -gt 0 ] || fail "no storage supports content type '$content'"
	if [ "${#items[@]}" -eq 2 ]; then
		echo "${items[0]}"
		return
	fi
	whiptail --backtitle "chanarr LXC" --title "$title" \
		--menu "Select storage:" 16 60 6 "${items[@]}" 3>&1 1>&2 2>&3
}

if [ -t 0 ] && command -v whiptail >/dev/null; then
	if ! whiptail --backtitle "chanarr LXC" --title "chanarr" \
		--yesno "Create a chanarr container with default settings?\n\n  CTID:      $CTID\n  Hostname:  $HOSTNAME\n  Disk:      ${DISK_GB}GB\n  Cores:     $CORES\n  RAM:       ${MEMORY_MB}MB\n  Network:   $BRIDGE / DHCP\n  Type:      unprivileged\n\nChoose <No> for Advanced Settings." 18 60; then
		CTID="$(ask "Container ID" "Container ID" "$CTID")"
		HOSTNAME="$(ask "Hostname" "Hostname" "$HOSTNAME")"
		DISK_GB="$(ask "Disk" "Disk size in GB" "$DISK_GB")"
		CORES="$(ask "CPU" "Number of cores" "$CORES")"
		MEMORY_MB="$(ask "RAM" "Memory in MB" "$MEMORY_MB")"
		BRIDGE="$(ask "Network" "Bridge" "$BRIDGE")"
		if ! whiptail --backtitle "chanarr LXC" --title "Network" --yesno "Use DHCP?" 8 40; then
			NET_IP="$(ask "Network" "Static IP in CIDR form (e.g. 192.168.1.50/24)" "")"
			GATEWAY="$(ask "Network" "Gateway (e.g. 192.168.1.1)" "")"
		fi
	fi
	ROOTFS_STORAGE="$(choose_storage rootdir "Container Storage")"
	TEMPLATE_STORAGE="$(choose_storage vztmpl "Template Storage")"
else
	# No terminal (piped run) — proceed with defaults, no dialogs.
	ROOTFS_STORAGE="$(pvesm status -content rootdir | awk 'NR==2 {print $1}')"
	TEMPLATE_STORAGE="$(pvesm status -content vztmpl | awk 'NR==2 {print $1}')"
fi

NET0="name=eth0,bridge=${BRIDGE},ip=${NET_IP}"
if [ "$NET_IP" != "dhcp" ] && [ -n "$GATEWAY" ]; then
	NET0="${NET0},gw=${GATEWAY}"
fi

# Debian 12 template, downloading the newest if none is present.
msg_info "Locating Debian 12 LXC template"
TEMPLATE="$(pveam list "$TEMPLATE_STORAGE" 2>/dev/null | awk '/debian-12-standard/{print $1}' | sort -V | tail -1)"
if [ -z "$TEMPLATE" ]; then
	pveam update >"$LOG" 2>&1 || fail "pveam update failed"
	LATEST="$(pveam available --section system | awk '/debian-12-standard/{print $2}' | sort -V | tail -1)"
	[ -n "$LATEST" ] || fail "no debian-12-standard template available"
	msg_info "Downloading $LATEST"
	pveam download "$TEMPLATE_STORAGE" "$LATEST" >"$LOG" 2>&1 || fail "template download failed"
	TEMPLATE="$TEMPLATE_STORAGE:vztmpl/$LATEST"
fi
msg_ok "Template: $TEMPLATE"

msg_info "Creating LXC container $CTID ($HOSTNAME)"
pct create "$CTID" "$TEMPLATE" \
	--hostname "$HOSTNAME" \
	--memory "$MEMORY_MB" \
	--cores "$CORES" \
	--rootfs "$ROOTFS_STORAGE:$DISK_GB" \
	--net0 "$NET0" \
	--unprivileged 1 \
	--features nesting=1 \
	--onboot 1 \
	--start 1 >"$LOG" 2>&1 || fail "pct create failed"
msg_ok "Created LXC container $CTID"

msg_info "Waiting for network in the container"
for _ in $(seq 1 30); do
	pct exec "$CTID" -- ping -c1 -W1 deb.debian.org >/dev/null 2>&1 && break
	sleep 2
done
pct exec "$CTID" -- ping -c1 -W1 deb.debian.org >/dev/null 2>&1 || fail "container never got network access"
msg_ok "Network is up"

msg_info "Installing ${APP} (this installs ffmpeg — takes a minute)"
# The debian-12-standard template ships without curl, so bootstrap it
# first; and fetch the install script to a file rather than process
# substitution — `bash <(curl ...)` runs an empty script (exit 0!) when
# the download fails, which once turned a failed install into a silent
# no-op reported as success.
pct exec "$CTID" -- bash -c "
	set -e
	export DEBIAN_FRONTEND=noninteractive
	apt-get update -qq
	apt-get install -y -qq --no-install-recommends curl ca-certificates >/dev/null
	curl -fsSL '$RAW/install/chanarr-install.sh' -o /tmp/chanarr-install.sh
	bash /tmp/chanarr-install.sh
" >"$LOG" 2>&1 || fail "install script failed"
pct exec "$CTID" -- systemctl is-active --quiet chanarr || fail "chanarr service is not running after install"
msg_ok "Installed ${APP}"

pct set "$CTID" --description "# chanarr

Folders of media, playing as live TV channels in Plex.

- Repo: https://github.com/${REPO}
- Update: run the install one-liner inside this container
" >/dev/null 2>&1 || true

IP="$(pct exec "$CTID" -- hostname -I | awk '{print $1}')"
echo
msg_ok "${APP} setup completed successfully!"
echo -e "${BL}  Web UI / Plex tuner:${CL} ${GN}http://${IP}:5004${CL}"
echo -e "${BL}  XMLTV guide:        ${CL} ${GN}http://${IP}:5004/epg.xml${CL}"
echo -e "${BL}  Logs:               ${CL} pct exec $CTID -- journalctl -u chanarr -f"
