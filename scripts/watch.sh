#!/usr/bin/env bash
# Continuously sync local source -> VM. Uses fswatch if installed (brew install fswatch),
# otherwise polls every second. air (Go) and Vite (frontend) on the VM pick up changes.
set -euo pipefail
cd "$(dirname "$0")/.."

sync() { make -s sync; }

sync
echo "[watch] initial sync done -> ${VM:-root@192.168.3.10}:${REMOTE:-/opt/daex}"

if command -v fswatch >/dev/null; then
  echo "[watch] using fswatch"
  fswatch -o -l 0.3 -e '/node_modules/' -e '/\.git/' -e '/tmp/' -e '/dist/' backend frontend deploy Makefile \
    | while read -r _; do sync && echo "[watch] synced $(date +%T)"; done
else
  echo "[watch] fswatch not found, polling every 1s (brew install fswatch for instant sync)"
  while true; do
    out=$(make -s sync RSYNC_FLAGS="-rlptzi" 2>&1 | grep -v '^\.' || true)
    [ -n "$out" ] && echo "[watch] $(date +%T)" && echo "$out"
    sleep 1
  done
fi
