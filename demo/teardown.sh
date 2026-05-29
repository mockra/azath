#!/usr/bin/env bash
# Tears down the azath demo environment.
set -euo pipefail
tmux kill-session -t api 2>/dev/null || true
tmux kill-session -t web 2>/dev/null || true
tmux kill-session -t docs 2>/dev/null || true
rm -rf /tmp/azath-demo
