#!/usr/bin/env bash
# Deploy backend Sales: tarik kode terbaru, build, jalankan/-ulang via PM2.
# Jalankan di server dari dalam folder repo: ./deploy.sh
set -euo pipefail
cd "$(dirname "$0")"

echo "==> git pull"
git pull --ff-only

echo "==> go build"
export PATH="$PATH:/usr/local/go/bin"
CGO_ENABLED=0 go build -trimpath -o sales-server ./cmd/server

# Muat env (port + koneksi Postgres) dari file di luar git: /opt/apps/sales.env
set -a; [ -f /opt/apps/sales.env ] && . /opt/apps/sales.env; set +a

echo "==> (re)start PM2: sales-be"
pm2 restart sales-be --update-env 2>/dev/null || pm2 start ./sales-server --name sales-be --update-env
pm2 save
echo "==> selesai. status:"
pm2 status sales-be
