package main

import (
	"fmt"
	"os"

	"github.com/mockra/azath/internal/config"
	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/state"
)

const usage = `azath - tmux + Copilot CLI orchestrator

Usage:
  azath up [project] [--fresh]    Start or re-enter a project's House.
                                  No arg opens the dashboard.
  azath down [project] [--kill]   Detach (or kill with --kill).
  azath list [--watch|--plain]    List projects with status.
  azath dash                      Open the dashboard (fzf picker).
  azath pick                      Run the fzf picker once (no tmux session).
  azath restart [--no-build]      Rebuild azath and recreate the dash session.
  azath show <project>            Print project details (used by picker preview).
  azath edit [project]            Open the editor for a project.
  azath resume [project]          Force cold-resume from saved session ID.
  azath logs [project] [-n N]     Print the last N lines from a project pane.
  azath kill-all [--yes]          Kill every running project session.
  azath sweep [--dry-run]         Kill sessions idle past idle-timeout.
  azath new <name> [path]         Append a [project.<name>] block to config.
  azath config                    Print the merged effective config.
  azath doctor                    Verify dependencies and paths.

Default (no args): azath dash.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "azath:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return cmdDash(nil)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "up":
		return cmdUp(rest)
	case "down":
		return cmdDown(rest)
	case "list", "ls":
		return cmdList(rest)
	case "dash":
		return cmdDash(rest)
	case "pick":
		return cmdPick(rest)
	case "restart":
		return cmdRestart(rest)
	case "show":
		return cmdShow(rest)
	case "edit":
		return cmdEdit(rest)
	case "resume":
		return cmdResume(rest)
	case "logs":
		return cmdLogs(rest)
	case "kill-all":
		return cmdKillAll(rest)
	case "sweep":
		return cmdSweep(rest)
	case "new":
		return cmdNew(rest)
	case "config":
		return cmdConfig()
	case "doctor":
		return cmdDoctor()
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Print(usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func loadAll() (config.Config, *state.State, []project.Project, error) {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return cfg, nil, nil, err
	}
	st, err := state.Load(cfg.StateFile)
	if err != nil {
		return cfg, nil, nil, err
	}
	projects, err := project.Resolve(cfg)
	if err != nil {
		return cfg, st, nil, err
	}
	return cfg, st, projects, nil
}
