#!/usr/bin/env bash
# Установка на Ubuntu/Debian VPS. Запуск: sudo ./deploy/install.sh
set -euo pipefail

APP_NAME=treesheild-newsbot
INSTALL_DIR=/opt/treesheild-newsbot
SERVICE_NAME=treesheild-newsbot.service
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ "${EUID:-}" -ne 0 ]]; then
  echo "Запустите от root: sudo $0"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Установите Go 1.24+ или соберите бинарник локально: make build-linux"
  exit 1
fi

id -u newsbot &>/dev/null || useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin newsbot

mkdir -p "$INSTALL_DIR"
cd "$ROOT_DIR"
CGO_ENABLED=0 go build -ldflags "-s -w" -o "$INSTALL_DIR/$APP_NAME" .

if [[ ! -f "$INSTALL_DIR/.env" ]]; then
  cp .env.example "$INSTALL_DIR/.env"
  chmod 600 "$INSTALL_DIR/.env"
  echo "Создан $INSTALL_DIR/.env — отредактируйте перед стартом."
fi

chown -R newsbot:newsbot "$INSTALL_DIR"
chmod 755 "$INSTALL_DIR/$APP_NAME"

cp "$SCRIPT_DIR/$SERVICE_NAME" /etc/systemd/system/
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"

echo ""
echo "Готово. Дальше:"
echo "  nano $INSTALL_DIR/.env"
echo "  sudo -u newsbot $INSTALL_DIR/$APP_NAME -preview"
echo "  systemctl start $SERVICE_NAME    # планировщик по CRON_SCHEDULE"
echo "  journalctl -u $SERVICE_NAME -f"
