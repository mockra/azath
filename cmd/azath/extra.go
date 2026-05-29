package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mockra/azath/internal/project"
	"github.com/mockra/azath/internal/tmux"
)

func cmdLogs(args []string) error {
	lines := 200
	var name string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--lines" || a == "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", a)
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v <= 0 {
				return fmt.Errorf("invalid line count %q", args[i])
			}
			lines = v
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %q", a)
		default:
			if name != "" {
				return fmt.Errorf("only one project at a time")
			}
			name = a
		}
	}
	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}
	if name == "" {
		name = tmux.CurrentSession()
		if name == "" || name == cfg.DashSession {
			return fmt.Errorf("usage: azath logs <project> [--lines N]")
		}
	}
	p, err := project.Find(cfg, name)
	if err != nil {
		return err
	}
	sessionName := tmuxNameFor(cfg, p.Name)
	if !tmux.HasSession(sessionName) {
		return fmt.Errorf("no running session for %q", name)
	}
	out, err := tmux.CapturePane(sessionName+":", lines)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

func cmdKillAll(args []string) error {
	yes := false
	for _, a := range args {
		switch a {
		case "--yes", "-y":
			yes = true
		default:
			return fmt.Errorf("unknown arg %q", a)
		}
	}
	cfg, st, projects, err := loadAll()
	if err != nil {
		return err
	}
	live := liveSessions()
	var targets []string
	for _, p := range projects {
		sessionName := tmuxNameFor(cfg, p.Name)
		if live[sessionName] && sessionName != cfg.DashSession {
			targets = append(targets, sessionName)
		}
	}
	if len(targets) == 0 {
		fmt.Println("no running project sessions")
		return nil
	}
	if !yes {
		fmt.Printf("Will kill %d sessions:\n", len(targets))
		for _, t := range targets {
			fmt.Printf("  %s\n", t)
		}
		fmt.Print("Proceed? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y") {
			fmt.Println("aborted")
			return nil
		}
	}
	for _, t := range targets {
		if err := tmux.KillSession(t); err != nil {
			fmt.Fprintf(os.Stderr, "kill %s: %v\n", t, err)
			continue
		}
		fmt.Printf("killed %s\n", t)
	}
	// Clear TmuxSession in state for anything we killed.
	for _, p := range projects {
		if s, ok := st.Get(p.Name); ok && s.TmuxSession != "" {
			if !tmux.HasSession(s.TmuxSession) {
				s.TmuxSession = ""
				st.Set(p.Name, s)
			}
		}
	}
	_ = st.Save()
	return nil
}

func cmdSweep(args []string) error {
	dryRun := false
	for _, a := range args {
		switch a {
		case "--dry-run", "-n":
			dryRun = true
		default:
			return fmt.Errorf("unknown arg %q", a)
		}
	}
	cfg, st, projects, err := loadAll()
	if err != nil {
		return err
	}
	live := liveSessions()
	now := time.Now()
	killed := 0
	for _, p := range projects {
		sessionName := tmuxNameFor(cfg, p.Name)
		if !live[sessionName] || sessionName == cfg.DashSession {
			continue
		}
		timeout := p.IdleTimeout
		if timeout == "" {
			continue
		}
		d, err := time.ParseDuration(timeout)
		if err != nil {
			continue
		}
		s, ok := st.Get(p.Name)
		if !ok || s.LastUsedAt.IsZero() {
			continue
		}
		idle := now.Sub(s.LastUsedAt)
		if idle < d {
			continue
		}
		if dryRun {
			fmt.Printf("would kill %s (idle %s, limit %s)\n", sessionName, idle.Round(time.Second), d)
			continue
		}
		if err := tmux.KillSession(sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "kill %s: %v\n", sessionName, err)
			continue
		}
		s.TmuxSession = ""
		st.Set(p.Name, s)
		killed++
		fmt.Printf("killed %s (idle %s)\n", sessionName, idle.Round(time.Second))
	}
	if killed > 0 {
		_ = st.Save()
	}
	if killed == 0 && !dryRun {
		fmt.Println("nothing to sweep")
	}
	return nil
}

func cmdNew(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: azath new <name> [path]")
	}
	name := args[0]
	if strings.ContainsAny(name, " \t/\"'") {
		return fmt.Errorf("invalid project name %q", name)
	}
	cfg, _, _, err := loadAll()
	if err != nil {
		return err
	}
	if _, exists := cfg.Projects[name]; exists {
		return fmt.Errorf("project %q already declared in %s", name, cfg.Path)
	}

	var path string
	if len(args) >= 2 {
		path = args[1]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = cwd
	}
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("path %s: %w", abs, err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(cfg.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	block := fmt.Sprintf("\n[project.%s]\npath = %q\n", name, abs)
	if _, err := f.WriteString(block); err != nil {
		return err
	}
	fmt.Printf("added [project.%s] -> %s in %s\n", name, abs, cfg.Path)
	return nil
}

func expandHome(p string) string {
	if p == "~" {
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), p[2:])
	}
	return p
}
