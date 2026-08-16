#!/usr/bin/env bash
# chanarr in-container install — executed inside the LXC by ct/chanarr.sh.
# Debian 12. Installs ffmpeg, fetches the latest chanarr release binary,
# and sets up the systemd service (data in /var/lib/chanarr).
#
# CHANARR_BINARY_URL overrides where the binary comes from (used by tests
# and pre-release verification).
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

REPO="brian-denton/chanarr"
BINARY_URL="${CHANARR_BINARY_URL:-https://github.com/${REPO}/releases/latest/download/chanarr-linux-amd64}"

echo "installing dependencies"
apt-get update -qq
apt-get install -y -qq --no-install-recommends ffmpeg ca-certificates curl >/dev/null

echo "creating chanarr user and data directory"
id chanarr >/dev/null 2>&1 || useradd --system --home-dir /var/lib/chanarr --shell /usr/sbin/nologin chanarr
mkdir -p /var/lib/chanarr
chown chanarr:chanarr /var/lib/chanarr

echo "downloading chanarr from $BINARY_URL"
curl -fsSL "$BINARY_URL" -o /usr/local/bin/chanarr
chmod 0755 /usr/local/bin/chanarr

echo "setting up the systemd service"
cat > /etc/systemd/system/chanarr.service <<'EOF'
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
systemctl daemon-reload
systemctl enable --now chanarr

echo "chanarr installed and running"
