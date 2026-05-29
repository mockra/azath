package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/tmux"
)

func cmdDash(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown arg %q", args[0])
	}
	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}

	dash := cfg.DashSession
	if !tmux.HasSession(dash) {
		bin, err := exec.LookPath("azath")
		if err != nil {
			bin = "azath"
		}
		// Loop forever so the dash session survives picker exits and errors.
		cmd := fmt.Sprintf("while true; do %s pick; sleep 0.5; done", bin)
		if err := tmux.NewDetached(dash, os.Getenv("HOME"), cmd); err != nil {
			return err
		}
	}
	return tmux.AttachOrSwitch(dash)
}

func cmdPick(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown arg %q", args[0])
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf not found in PATH; install fzf to use the picker")
	}
	bin, err := exec.LookPath("azath")
	if err != nil {
		bin = "azath"
	}
	reload := fmt.Sprintf("%s list --plain", bin)
	preview := fmt.Sprintf("%s show {2}", bin)

	for {
		input, err := exec.Command(bin, "list", "--plain").Output()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		if len(strings.TrimSpace(string(input))) == 0 {
			fmt.Fprintln(os.Stderr, "azath: no projects discovered")
			return nil
		}

		fzfArgs := []string{
			"--ansi", "--reverse", "--no-sort", "--no-mouse", "--cycle",
			"--prompt", "azath> ",
			"--delimiter", `[[:space:]]+`, "--nth", "2",
			"--preview", preview,
			"--preview-window", "right:50%:wrap",
			"--header", "enter: up   ^e: edit   ^x: kill   ^r: refresh   esc: last",
			"--bind", fmt.Sprintf("ctrl-x:execute-silent(%s down --kill {2})+reload(%s)", bin, reload),
			"--bind", fmt.Sprintf("ctrl-r:reload(%s)", reload),
			"--bind", fmt.Sprintf("ctrl-e:execute-silent(%s up {2} >/dev/null 2>&1; %s edit {2} >/dev/null 2>&1)+reload(%s)", bin, bin, reload),
			"--bind", "esc:execute-silent(tmux switch-client -l)",
		}

		fzf := exec.Command("fzf", fzfArgs...)
		fzf.Stdin = strings.NewReader(string(input))
		fzf.Stderr = os.Stderr
		out, err := fzf.Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				switch ee.ExitCode() {
				case 130, 1:
					return nil
				}
			}
			return err
		}
		line := strings.TrimSpace(string(out))
		if line == "" {
			return nil
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil
		}
		if err := cmdUp([]string{fields[1]}); err != nil {
			fmt.Fprintln(os.Stderr, "azath:", err)
		}
	}
}

func cmdRestart(args []string) error {
	build := true
	for _, a := range args {
		switch a {
		case "--no-build":
			build = false
		default:
			return fmt.Errorf("unknown arg %q", a)
		}
	}

	if build {
		dir, err := sourceDir()
		if err != nil {
			return fmt.Errorf("locate azath source: %w", err)
		}
		fmt.Printf("Building azath in %s...\n", dir)
		cmd := exec.Command("make", "install")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("rebuild: %w", err)
		}
	}

	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}

	if tmux.CurrentSession() == cfg.DashSession {
		return fmt.Errorf("cannot restart while attached to %q; run from another tmux session or outside tmux", cfg.DashSession)
	}

	if tmux.HasSession(cfg.DashSession) {
		if err := tmux.KillSession(cfg.DashSession); err != nil {
			return err
		}
		fmt.Printf("Killed %s.\n", cfg.DashSession)
	}

	return cmdDash(nil)
}

func sourceDir() (string, error) {
	bin, err := exec.LookPath("azath")
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(bin)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(real)
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); err != nil {
		return "", fmt.Errorf("no Makefile at %s", dir)
	}
	return dir, nil
}

func cmdShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project name required")
	}
	name := args[0]
	cfg, st, _, err := loadAll()
	if err != nil {
		return err
	}
	p, err := project.Find(cfg, name)
	if err != nil {
		return err
	}
	sessionName := tmuxNameFor(cfg, p.Name)
	running := tmux.HasSession(sessionName)
	info := collectDashInfo(cfg, st, p, sessionName, running)
	fmt.Printf("%s\n", p.Name)
	fmt.Printf("status   %s\n", activityLabel(info.activity))
	if info.unread {
		fmt.Printf("unread   yes\n")
	}
	fmt.Printf("path     %s\n", p.Path)
	fmt.Printf("agent    %s\n", p.AgentCommand)
	fmt.Printf("editor   %s (%s)\n", p.Editor, p.EditorPlacement)
	fmt.Printf("start    %s\n", p.StartWith)
	if p.IdleTimeout != "" {
		fmt.Printf("idle     %s\n", p.IdleTimeout)
	}
	if s, ok := st.Get(p.Name); ok {
		if !s.LastUsedAt.IsZero() {
			fmt.Printf("used     %s\n", humanize(s.LastUsedAt))
		}
		if s.CopilotSessionID != "" {
			fmt.Printf("session  %s\n", s.CopilotSessionID)
		}
	}
	if !info.lastEvents.IsZero() {
		fmt.Printf("activity %s\n", humanize(info.lastEvents))
	}
	if info.lastMessage != "" {
		fmt.Println()
		fmt.Println("last prompt")
		fmt.Printf("  %s\n", info.lastMessage)
	}
	if running {
		if wins, err := tmux.ListWindows(sessionName); err == nil && len(wins) > 0 {
			fmt.Println()
			fmt.Println("windows")
			for _, w := range wins {
				fmt.Printf("  %s\n", w)
			}
		}
	}
	return nil
}
