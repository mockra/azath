# azath

A tmux + Copilot CLI orchestrator. Each project lives in its own House (tmux
session) with persistent agent sessions that survive detach, terminal restarts,
and reboots.

## Install

```sh
cd ~/code/azath
make install   # builds and symlinks to ~/bin/azath
```

## Quick start

```sh
azath doctor                 # verify deps
azath list                   # list discovered projects
azath up issues              # start or re-enter the issues House
azath edit                   # open editor in the current House
azath dash                   # open the dashboard
```

## Config

Default location: `~/.config/azath/config.toml`

```toml
agent-command = "copilot"
editor = "nvim"
editor-placement = "window"        # window | pane-right | pane-bottom
start-with = "agent"               # agent | editor | shell
projects-root = ["~/code", "~/github"]
auto-discover = true
exclude = ["~/code/exercism"]

[project.issues]
path = "~/code/issues"
agent-command = "copilot --model claude-opus-4.6"
editor-placement = "pane-right"

[project.github]
path = "~/github/github"
agent-command = "github-dev"
start-with = "editor"               # skip the agent; open nvim in the first window

[project.home]
path = "~"
start-with = "shell"                # plain login shell
```

## State

Managed at `~/.local/share/azath/state.toml`. Tracks active tmux sessions and
the most recent Copilot session ID per project so a reboot can cold-resume
with `copilot --resume <id>`.

## Commands

| Command | Description |
| --- | --- |
| `azath up [project]` | Start or re-enter a House. No arg opens the dashboard. |
| `azath down [project]` | Detach the House. `--kill` to terminate. |
| `azath list [--watch\|--plain]` | List projects with status. `--plain` emits a colored, picker-friendly format. |
| `azath dash` | Attach the `azath-dash` dashboard. |
| `azath pick` | Run the fzf picker once without a tmux session. |
| `azath restart [--no-build]` | Rebuild via `make install` and recreate the dash session. |
| `azath show <project>` | Print project details (used by the picker preview). |
| `azath edit [project]` | Open editor in the House per `editor-placement`. |
| `azath resume [project]` | Force cold-resume from saved Copilot session ID. |
| `azath logs [project] [-n N]` | Print the last N lines of the project's main pane (default 200). |
| `azath kill-all [--yes]` | Kill every running project session (skips `azath-dash`). |
| `azath sweep [--dry-run]` | Kill sessions whose `LastUsedAt` exceeds `idle-timeout`. |
| `azath new <name> [path]` | Append a `[project.<name>]` block to your config. |
| `azath config` | Print the merged effective config. |
| `azath doctor` | Verify tmux, copilot, nvim, fzf, and config paths. |

## Dashboard

`azath dash` launches an fzf picker inside the `azath-dash` tmux session showing
every project with a live up/down indicator (running projects sort to the top).
Bindings:

- `enter` start or switch to the selected project
- `ctrl-e` open the editor in the selected project's House
- `ctrl-x` kill the selected project's tmux session
- `ctrl-r` refresh the project list
- `esc` switch tmux to the last-attached session

A preview pane on the right shows status, path, agent, editor placement,
saved Copilot session ID, and current tmux windows for the selected project.
Requires `fzf` on `PATH`.

## Idle timeout

Set `idle-timeout` globally or per-project (e.g. `"4h"`) and run `azath sweep`
(manually or from cron/launchd) to kill sessions whose `LastUsedAt` exceeds
the threshold. Use `--dry-run` to preview first.
