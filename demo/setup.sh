#!/usr/bin/env bash
# Sets up a self-contained azath demo environment under /tmp/azath-demo.
set -euo pipefail

ROOT=/tmp/azath-demo
rm -rf "$ROOT"
mkdir -p "$ROOT/.config/azath" "$ROOT/.local/share/azath" "$ROOT/code"

for p in api web docs; do
  mkdir -p "$ROOT/code/$p"
done

cat > "$ROOT/.config/azath/config.toml" <<'TOML'
projects-root = ["~/code"]
idle-timeout = "4h"

[project.api]
path = "~/code/api"
agent-command = "echo demo api agent"
start-with = "agent"

[project.web]
path = "~/code/web"
agent-command = "echo demo web agent"

[project.docs]
path = "~/code/docs"
agent-command = "echo demo docs agent"
start-with = "editor"
TOML

export HOME="$ROOT"
export PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd):$PATH"
cd "$HOME"

# Pretend some projects are running so list shows status variety.
tmux kill-session -t api 2>/dev/null || true
tmux new-session -d -s api -c "$HOME/code/api" 'sleep 999999'
tmux kill-session -t web 2>/dev/null || true
tmux new-session -d -s web -c "$HOME/code/web" 'sleep 999999'
